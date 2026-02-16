package services

import (
	"context"
	"sync"
	"time"

	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"go.uber.org/zap"
)

type EventOrderingService struct {
	db            database.DatabaseInterface
	processFunc   func(*models.OrderedEvent) error
	flushInterval time.Duration
	maxAge        time.Duration
	batchSize     int
	mutex         sync.Mutex
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewEventOrderingService(db database.DatabaseInterface, processFunc func(*models.OrderedEvent) error) *EventOrderingService {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventOrderingService{
		db:            db,
		processFunc:   processFunc,
		flushInterval: 5 * time.Second,
		maxAge:        10 * time.Second,
		batchSize:     100,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (s *EventOrderingService) Start() {
	s.wg.Add(1)
	go s.flushWorker()
}

func (s *EventOrderingService) Stop() {
	s.cancel()
	s.wg.Wait()
}

func (s *EventOrderingService) AddEvent(event *models.OrderedEvent) error {
	return s.db.StoreWebhookEvent(s.ctx, event)
}

func (s *EventOrderingService) flushWorker() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
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

	events, err := s.db.GetPendingEventsGrouped(s.ctx, 1000)
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
