package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gateixeira/live-actions/pkg/metrics"
	"go.uber.org/zap"
)

// ErrIngestQueueFull is returned by AddEvent when the in-memory ingest queue
// remains full for longer than enqueueTimeout. The HTTP handler should treat
// this as a 5xx so operators can investigate; GitHub does not auto-retry
// webhook deliveries, so the affected event will need manual redelivery.
var ErrIngestQueueFull = errors.New("event ingest queue full")

// ErrPermanent wraps errors that are known to be unrecoverable: bad payload,
// schema mismatch, unknown event type. Events that fail with this sentinel
// MUST NOT be retried — the failure would just repeat on every replay.
//
// ErrTransient wraps errors that are expected to succeed on retry: SQLite
// busy/locked, transient I/O. Events that fail with this sentinel are
// spilled to webhook_events so the cold-path flush worker retries them.
//
// Any error that is neither classified is treated as transient (spill +
// retry) — that's the safe default; an event silently dropped is much worse
// than one extra cold-path retry.
var (
	ErrPermanent = errors.New("permanent processing error")
	ErrTransient = errors.New("transient processing error")
)

// IsPermanent reports whether err should NOT be retried.
func IsPermanent(err error) bool {
	return errors.Is(err, ErrPermanent)
}

// Default sizing for the async ingest pipeline. These values trade a small
// shutdown-loss window for a large throughput improvement: events accepted
// from GitHub live in memory until the next batch flush.
const (
	defaultIngestChannelSize = 10000
	defaultIngestBatchSize   = 200
	defaultIngestBatchWait   = 50 * time.Millisecond
	// 8s leaves headroom under GitHub's ~10s webhook delivery timeout, so the
	// HTTP request is still answered if the ingest pipeline becomes saturated.
	defaultEnqueueTimeout = 8 * time.Second
)

type EventOrderingService struct {
	db          database.DatabaseInterface
	processFunc func(*models.OrderedEvent) error

	// flush worker (replays persisted events through processFunc)
	flushInterval time.Duration
	maxAge        time.Duration
	batchSize     int

	// ingest worker (buffers AddEvent calls and batch-INSERTs them)
	ingestCh         chan *models.OrderedEvent
	ingestBatchSize  int
	ingestBatchWait  time.Duration
	enqueueTimeout   time.Duration
	ingestDrainWait  time.Duration
	ingestDoneCh     chan struct{}

	mutex  sync.Mutex
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func NewEventOrderingService(db database.DatabaseInterface, processFunc func(*models.OrderedEvent) error) *EventOrderingService {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventOrderingService{
		db:               db,
		processFunc:      processFunc,
		flushInterval:    5 * time.Second,
		maxAge:           10 * time.Second,
		batchSize:        500,
		ingestCh:         make(chan *models.OrderedEvent, defaultIngestChannelSize),
		ingestBatchSize:  defaultIngestBatchSize,
		ingestBatchWait:  defaultIngestBatchWait,
		enqueueTimeout:   defaultEnqueueTimeout,
		ingestDrainWait:  5 * time.Second,
		ingestDoneCh:     make(chan struct{}),
		ctx:              ctx,
		cancel:           cancel,
	}
}

func (s *EventOrderingService) Start() {
	s.wg.Add(2)
	go s.ingestWorker()
	go s.flushWorker()
}

// IngestQueueLen returns the current number of events buffered in the
// in-memory ingest channel. Intended for observability/health endpoints.
func (s *EventOrderingService) IngestQueueLen() int {
	return len(s.ingestCh)
}

// IngestQueueCap returns the configured capacity of the in-memory ingest
// channel. Intended for observability/health endpoints.
func (s *EventOrderingService) IngestQueueCap() int {
	return cap(s.ingestCh)
}

// Stop signals both workers to drain and exit, then blocks until they do.
// Order is enforced inside the goroutines: the flush worker waits for the
// ingest worker to finish persisting any in-memory events before running its
// final flushAll, so events queued at shutdown are not lost between layers.
func (s *EventOrderingService) Stop() {
	s.cancel()
	s.wg.Wait()
}

// AddEvent enqueues an event for asynchronous batched persistence. It blocks
// for up to enqueueTimeout if the in-memory channel is full, applying
// back-pressure on GitHub's HTTP client during bursts. Returns
// ErrIngestQueueFull on timeout (or context.Canceled if the service is
// shutting down) so the HTTP handler can return 5xx.
func (s *EventOrderingService) AddEvent(event *models.OrderedEvent) error {
	// Fast path: non-blocking send.
	select {
	case s.ingestCh <- event:
		return nil
	default:
	}

	if s.enqueueTimeout <= 0 {
		// Block indefinitely (only safe if the caller has its own deadline).
		select {
		case s.ingestCh <- event:
			return nil
		case <-s.ctx.Done():
			return s.ctx.Err()
		}
	}

	timer := time.NewTimer(s.enqueueTimeout)
	defer timer.Stop()
	select {
	case s.ingestCh <- event:
		return nil
	case <-timer.C:
		logger.Logger.Warn("Webhook ingest channel full; manual redelivery may be needed",
			zap.String("delivery_id", event.Sequence.DeliveryID),
			zap.String("event_type", event.EventType),
			zap.Duration("waited", s.enqueueTimeout),
			zap.Int("channel_capacity", cap(s.ingestCh)),
		)
		return ErrIngestQueueFull
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

// ingestWorker drains ingestCh and processes each event in arrival order on
// the happy path: processFunc is called directly and the event never touches
// webhook_events. Only events whose processing fails with a non-permanent
// error are spilled to webhook_events for the cold-path flush worker to
// retry — and those spills are batched.
//
// On context cancellation the worker drains any remaining buffered events
// before signalling ingestDoneCh so the flush worker can run its final
// flushAll against an up-to-date webhook_events table.
func (s *EventOrderingService) ingestWorker() {
	defer s.wg.Done()
	defer close(s.ingestDoneCh)

	spill := make([]*models.OrderedEvent, 0, s.ingestBatchSize)
	ticker := time.NewTicker(s.ingestBatchWait)
	defer ticker.Stop()

	flushSpill := func(ctx context.Context) {
		if len(spill) == 0 {
			return
		}
		if err := s.db.StoreWebhookEvents(ctx, spill); err != nil {
			// We failed to spill — there is no further safety net, so log
			// loudly. The events are lost (in-memory only) unless GitHub
			// resends them via manual redelivery.
			logger.Logger.Error("Failed to spill webhook events for retry; events dropped",
				zap.Int("batch_size", len(spill)),
				zap.Error(err),
			)
			for _, ev := range spill {
				metrics.GetRegistry().WebhookEventsTotal.
					WithLabelValues(ev.EventType, "spill_failed").Inc()
			}
		} else {
			logger.Logger.Debug("Spilled events for cold-path retry", zap.Int("batch_size", len(spill)))
			for _, ev := range spill {
				metrics.GetRegistry().WebhookEventsTotal.
					WithLabelValues(ev.EventType, "spilled").Inc()
			}
		}
		spill = spill[:0]
	}

	process := func(ev *models.OrderedEvent) {
		err := s.processFunc(ev)
		if err == nil {
			return
		}
		if IsPermanent(err) {
			logger.Logger.Warn("Permanent processing failure; event dropped",
				zap.String("event_type", ev.EventType),
				zap.String("delivery_id", ev.Sequence.DeliveryID),
				zap.String("ordering_key", ev.OrderingKey),
				zap.Error(err))
			metrics.GetRegistry().WebhookEventsTotal.
				WithLabelValues(ev.EventType, "permanent_failure").Inc()
			return
		}
		// Transient (or unclassified): spill so the flush worker retries.
		logger.Logger.Warn("Transient processing failure; spilling to webhook_events",
			zap.String("event_type", ev.EventType),
			zap.String("delivery_id", ev.Sequence.DeliveryID),
			zap.Error(err))
		spill = append(spill, ev)
	}

	for {
		select {
		case <-s.ctx.Done():
			// Drain remaining events using a fresh context so writes can complete
			// even though the service context is cancelled.
			drainCtx, cancel := context.WithTimeout(context.Background(), s.ingestDrainWait)
			defer cancel()
			for {
				select {
				case ev := <-s.ingestCh:
					process(ev)
					if len(spill) >= s.ingestBatchSize {
						flushSpill(drainCtx)
					}
				default:
					flushSpill(drainCtx)
					return
				}
			}
		case ev := <-s.ingestCh:
			process(ev)
			if len(spill) >= s.ingestBatchSize {
				flushSpill(s.ctx)
			}
		case <-ticker.C:
			flushSpill(s.ctx)
		}
	}
}

func (s *EventOrderingService) flushWorker() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			// Wait for the ingest worker to finish persisting in-memory events
			// before attempting the final flush, otherwise pending rows from
			// the last burst would be missed.
			<-s.ingestDoneCh
			s.flushAll()
			return
		case <-ticker.C:
			s.flushReadyEvents()
		}
	}
}

// drainBudget bounds how long flushReadyEvents will keep pulling batches
// before yielding back to the ticker. This caps mutex/writer monopolisation
// so the ingest worker keeps its share of the single-writer SQLite pool.
const drainBudget = 500 * time.Millisecond

// maxDrainIterations is a hard ceiling on how many batches a single tick
// will pull, guarding against pathological loops if a query keeps returning
// the configured batchSize.
const maxDrainIterations = 20

func (s *EventOrderingService) flushReadyEvents() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	start := time.Now()
	processed := 0
	defer func() {
		reg := metrics.GetRegistry()
		reg.FlushBatchDurationSeconds.Observe(time.Since(start).Seconds())
		if processed > 0 {
			reg.FlushBatchEvents.Observe(float64(processed))
		}
	}()

	deadline := time.Now().Add(drainBudget)
	for i := 0; i < maxDrainIterations; i++ {
		if s.ctx.Err() != nil {
			return
		}
		events, err := s.db.GetPendingEventsByAge(s.ctx, s.maxAge, s.batchSize)
		if err != nil {
			logger.Logger.Error("Failed to fetch pending events", zap.Error(err))
			return
		}
		if len(events) == 0 {
			return
		}
		logger.Logger.Debug("Processing batch of pending events",
			zap.Int("count", len(events)),
			zap.Int("iteration", i))
		s.processEvents(events)
		processed += len(events)

		// Stop early when the backlog is drained or our time slice is up.
		if len(events) < s.batchSize || time.Now().After(deadline) {
			return
		}
	}
}

func (s *EventOrderingService) flushAll() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Use a fresh context: s.ctx is already cancelled at this point.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		if ctx.Err() != nil {
			logger.Logger.Warn("flushAll deadline reached with events still pending")
			return
		}
		events, err := s.db.GetPendingEventsGrouped(ctx, 1000)
		if err != nil {
			logger.Logger.Error("Failed to fetch all pending events", zap.Error(err))
			return
		}
		if len(events) == 0 {
			return
		}
		logger.Logger.Debug("Processing all pending events",
			zap.Int("count", len(events)))
		s.processEvents(events)
		if len(events) < 1000 {
			return
		}
	}
}

func (s *EventOrderingService) processEvents(events []*models.OrderedEvent) {
	for _, event := range events {
		if err := s.processFunc(event); err != nil {
			logger.Logger.Error("Failed to process event",
				zap.String("event_type", event.EventType),
				zap.String("delivery_id", event.Sequence.DeliveryID),
				zap.String("ordering_key", event.OrderingKey),
				zap.Int("status_priority", event.StatusPriority),
				zap.Error(err))
			continue
		}

		logger.Logger.Debug("Event processed successfully",
			zap.String("event_type", event.EventType),
			zap.String("delivery_id", event.Sequence.DeliveryID),
			zap.String("ordering_key", event.OrderingKey),
			zap.Int("status_priority", event.StatusPriority))
	}
}
