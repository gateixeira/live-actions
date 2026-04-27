package handlers

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
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

// SSEHandler handles server-sent events
type SSEHandler struct {
	client    chan SSEEvent
	coalescer *sseCoalescer
}

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
			client: make(chan SSEEvent, 100),
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
	if h.client == nil {
		return
	}
	select {
	case h.client <- event:
		logger.Logger.Debug("SSE event sent", zap.String("type", event.Type))
	default:
		logger.Logger.Debug("SSE channel full, dropping event", zap.String("type", event.Type))
	}
}

func (h *SSEHandler) HandleSSE() gin.HandlerFunc {
	return func(c *gin.Context) {

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		clientChan := make(chan SSEEvent, 100)

		go func() {
			for {
				select {
				case event := <-h.client:
					select {
					case clientChan <- event:
					default:
						// Client channel full, skip this event
					}
				case <-c.Request.Context().Done():
					// Client disconnected
					close(clientChan)
					return
				}
			}
		}()

		// Send initial connection event
		c.SSEvent("message", map[string]interface{}{
			"type": "connected",
			"data": map[string]string{
				"message":   "SSE connection established",
				"timestamp": time.Now().Format(time.RFC3339),
			},
		})

		// Keep connection alive and send events
		for {
			select {
			case event, ok := <-clientChan:
				if !ok {
					// Channel closed, client disconnected
					return
				}

				jsonData, err := json.Marshal(event)
				if err != nil {
					logger.Logger.Error("Failed to marshal SSE event", zap.Error(err))
					continue
				}

				c.SSEvent("message", string(jsonData))
				c.Writer.Flush()

			case <-c.Request.Context().Done():
				// Client disconnected
				logger.Logger.Debug("SSE client disconnected")
				return

			case <-time.After(30 * time.Second):
				// Send keepalive ping
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
