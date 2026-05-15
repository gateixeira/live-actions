package handlers

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gateixeira/live-actions/pkg/metrics"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SSEEvent represents a server-sent event
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// defaultCoalesceInterval bounds how often we emit a given event type to
// connected SSE clients. Under high webhook throughput this collapses bursts
// (one per processed event) into at most ~2 emits/sec/type, keeping only the
// most recent payload. The UI re-fetches detail data via the REST API on each
// update, so dropping intermediate states is safe.
const defaultCoalesceInterval = 500 * time.Millisecond

// SSEHandler handles server-sent events. It maintains a registry of connected
// clients and fans every emitted event out to all of them.
type SSEHandler struct {
	coalescer *sseCoalescer

	mu   sync.Mutex
	subs map[*sseSubscriber]struct{}
}

// sseSubscriber represents a single connected SSE client. Each connected
// browser gets its own buffered channel so a slow client cannot starve other
// clients of events.
type sseSubscriber struct {
	ch chan SSEEvent
}

// subscriberBufferSize bounds how many pending events we will queue per
// connected client before dropping. With the coalescer in front emitting at
// most ~2/sec/type, this is large enough to absorb GC pauses or brief
// network slowness without dropping under normal load.
const subscriberBufferSize = 100

// sseCoalescer keeps only the latest payload per event type and emits at most
// once per interval via a single ticker goroutine.
type sseCoalescer struct {
	mu       sync.Mutex
	pending  map[string]SSEEvent
	interval time.Duration
	sender   func(SSEEvent)
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

func newSSECoalescer(interval time.Duration, sender func(SSEEvent)) *sseCoalescer {
	c := &sseCoalescer{
		pending:  make(map[string]SSEEvent),
		interval: interval,
		sender:   sender,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	go c.run()
	return c
}

func (c *sseCoalescer) run() {
	defer close(c.doneCh)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			c.flush()
			return
		case <-ticker.C:
			c.flush()
		}
	}
}

func (c *sseCoalescer) submit(ev SSEEvent) {
	c.mu.Lock()
	c.pending[ev.Type] = ev
	c.mu.Unlock()
}

func (c *sseCoalescer) flush() {
	c.mu.Lock()
	if len(c.pending) == 0 {
		c.mu.Unlock()
		return
	}
	events := make([]SSEEvent, 0, len(c.pending))
	for _, e := range c.pending {
		events = append(events, e)
	}
	c.pending = make(map[string]SSEEvent)
	c.mu.Unlock()
	for _, e := range events {
		c.sender(e)
	}
}

func (c *sseCoalescer) stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.doneCh
}

// Global SSE handler instance
var (
	sseHandler *SSEHandler
	sseOnce    sync.Once
)

func InitSSEHandler() {
	sseOnce.Do(func() {
		h := &SSEHandler{
			subs: make(map[*sseSubscriber]struct{}),
		}
		h.coalescer = newSSECoalescer(defaultCoalesceInterval, h.sendEventNow)
		sseHandler = h
	})
}

func GetSSEHandler() *SSEHandler {
	InitSSEHandler()
	return sseHandler
}

// SendEvent emits an event immediately. Use SendEventCoalesced for high-volume
// event types where intermediate states are not needed (e.g. metrics_update,
// workflow_update under load).
func (h *SSEHandler) SendEvent(eventType string, data interface{}) {
	if h == nil {
		return
	}
	h.sendEventNow(SSEEvent{Type: eventType, Data: data})
}

// SendEventCoalesced records the latest payload for the given event type; the
// coalescer ticker emits the most recent one at most once per interval.
func (h *SSEHandler) SendEventCoalesced(eventType string, data interface{}) {
	if h == nil || h.coalescer == nil {
		return
	}
	h.coalescer.submit(SSEEvent{Type: eventType, Data: data})
}

func (h *SSEHandler) sendEventNow(event SSEEvent) {
	if h == nil {
		return
	}
	// Snapshot subscribers under the lock so a slow consumer cannot block
	// fanout to its peers (the actual send happens outside the lock).
	h.mu.Lock()
	subs := make([]*sseSubscriber, 0, len(h.subs))
	for s := range h.subs {
		subs = append(subs, s)
	}
	h.mu.Unlock()

	reg := metrics.GetRegistry()
	reg.SSEEventsBroadcastTotal.WithLabelValues(event.Type).Inc()

	for _, s := range subs {
		select {
		case s.ch <- event:
		default:
			// Per-client buffer full: drop this event for that client.
			// They will catch up on the next REST poll.
			reg.SSEEventsDroppedTotal.WithLabelValues(event.Type).Inc()
			logger.Logger.Debug("SSE subscriber buffer full, dropping event",
				zap.String("type", event.Type))
		}
	}
}

// subscribe registers a new connected client and returns its subscription.
// Callers must invoke unsubscribe when the connection ends so the entry is
// removed from the registry.
func (h *SSEHandler) subscribe() *sseSubscriber {
	s := &sseSubscriber{ch: make(chan SSEEvent, subscriberBufferSize)}
	h.mu.Lock()
	if h.subs == nil {
		h.subs = make(map[*sseSubscriber]struct{})
	}
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

func (h *SSEHandler) unsubscribe(s *sseSubscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
	// We deliberately do NOT close s.ch here. sendEventNow may have captured a
	// reference to this subscriber in its snapshot before we removed it; if we
	// closed the channel, the concurrent send would panic. The channel is
	// orphaned and will be garbage-collected once sendEventNow's snapshot is
	// released.
}

// SubscriberCount reports how many clients are currently connected. Intended
// for tests and observability (exposed as a Prometheus GaugeFunc at startup).
func (h *SSEHandler) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// subscriberCount is retained as a lower-case alias for older test call sites.
func (h *SSEHandler) subscriberCount() int { return h.SubscriberCount() }

func (h *SSEHandler) HandleSSE() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		sub := h.subscribe()
		defer h.unsubscribe(sub)

		// Send initial connection event
		c.SSEvent("message", map[string]interface{}{
			"type": "connected",
			"data": map[string]string{
				"message":   "SSE connection established",
				"timestamp": time.Now().Format(time.RFC3339),
			},
		})
		c.Writer.Flush()

		keepalive := time.NewTicker(30 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case event := <-sub.ch:
				jsonData, err := json.Marshal(event)
				if err != nil {
					logger.Logger.Error("Failed to marshal SSE event", zap.Error(err))
					continue
				}
				c.SSEvent("message", string(jsonData))
				c.Writer.Flush()

			case <-c.Request.Context().Done():
				logger.Logger.Debug("SSE client disconnected")
				return

			case <-keepalive.C:
				c.SSEvent("ping", map[string]string{
					"timestamp": time.Now().Format(time.RFC3339),
				})
				c.Writer.Flush()
			}
		}
	}
}

// SendMetricsUpdate sends a metrics update event. Coalesced because under
// load this is fired once per processed job event.
func SendMetricsUpdate(update models.MetricsUpdateEvent) {
	if sseHandler != nil {
		sseHandler.SendEventCoalesced("metrics_update", update)
	}
}

// SendWorkflowUpdate sends a workflow update event. Coalesced because under
// load this is fired once per processed run event; the UI re-fetches the
// table on receipt, so intermediate states can be dropped safely.
func SendWorkflowUpdate(update models.WorkflowUpdateEvent) {
	if sseHandler != nil {
		sseHandler.SendEventCoalesced("workflow_update", update)
	}
}
