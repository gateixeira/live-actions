package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"go.uber.org/zap"
)

// ErrIngestQueueFull is returned by AddEvent when the in-memory ingest queue
// remains full for longer than enqueueTimeout. The HTTP handler should treat
// this as a 5xx so operators can investigate; GitHub does not auto-retry
// webhook deliveries, so the affected event will need manual redelivery.
var ErrIngestQueueFull = errors.New("event ingest queue full")

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
		batchSize:        100,
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

// ingestWorker drains ingestCh and persists events in batched transactions.
// On context cancellation it drains any remaining buffered events before
// signalling ingestDoneCh so the flush worker can run its final flushAll
// against an up-to-date webhook_events table.
func (s *EventOrderingService) ingestWorker() {
	defer s.wg.Done()
	defer close(s.ingestDoneCh)

	batch := make([]*models.OrderedEvent, 0, s.ingestBatchSize)
	ticker := time.NewTicker(s.ingestBatchWait)
	defer ticker.Stop()

	flush := func(ctx context.Context) {
		if len(batch) == 0 {
			return
		}
		if err := s.db.StoreWebhookEvents(ctx, batch); err != nil {
			logger.Logger.Error("Batched webhook event insert failed",
				zap.Int("batch_size", len(batch)),
				zap.Error(err),
			)
		} else {
			logger.Logger.Debug("Persisted webhook event batch", zap.Int("batch_size", len(batch)))
		}
		batch = batch[:0]
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
					batch = append(batch, ev)
					if len(batch) >= s.ingestBatchSize {
						flush(drainCtx)
					}
				default:
					flush(drainCtx)
					return
				}
			}
		case ev := <-s.ingestCh:
			batch = append(batch, ev)
			if len(batch) >= s.ingestBatchSize {
				flush(s.ctx)
			}
		case <-ticker.C:
			flush(s.ctx)
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

func (s *EventOrderingService) flushReadyEvents() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	events, err := s.db.GetPendingEventsByAge(s.ctx, s.maxAge, s.batchSize)
	if err != nil {
		logger.Logger.Error("Failed to fetch pending events", zap.Error(err))
		return
	}

	if len(events) > 0 {
		logger.Logger.Debug("Processing batch of pending events",
			zap.Int("count", len(events)))
		s.processEvents(events)
	}
}

func (s *EventOrderingService) flushAll() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Use a fresh context: s.ctx is already cancelled at this point.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, err := s.db.GetPendingEventsGrouped(ctx, 1000)
	if err != nil {
		logger.Logger.Error("Failed to fetch all pending events", zap.Error(err))
		return
	}

	if len(events) > 0 {
		logger.Logger.Debug("Processing all pending events",
			zap.Int("count", len(events)))
		s.processEvents(events)
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
