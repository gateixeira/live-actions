package services

import (
	"context"
	"sync"
	"time"

	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gateixeira/live-actions/pkg/metrics"
	"go.uber.org/zap"
)

type MetricsUpdateService struct {
	db       database.DatabaseInterface
	registry *metrics.Registry
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	mutex    sync.RWMutex
}

func NewMetricsUpdateService(db database.DatabaseInterface, interval time.Duration, ctx context.Context) *MetricsUpdateService {
	ctx, cancel := context.WithCancel(ctx)

	return &MetricsUpdateService{
		db:       db,
		registry: metrics.GetRegistry(),
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
}

func (s *MetricsUpdateService) Start() {
	defer close(s.done)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Update immediately on start
	s.updateMetrics()

	for {
		select {
		case <-s.ctx.Done():
			logger.Logger.Info("Metrics update service stopped")
			return
		case <-ticker.C:
			s.updateMetrics()
		}
	}
}

func (s *MetricsUpdateService) Stop() {
	s.cancel()
	<-s.done // Wait for completion
}

func (s *MetricsUpdateService) updateMetrics() {
	// Lock to prevent concurrent updates
	s.mutex.Lock()
	defer s.mutex.Unlock()

	running, queued, err := s.db.GetCurrentJobCounts()
	if err != nil {
		logger.Logger.Error("Failed to get current job counts", zap.Error(err))
		return
	}

	s.registry.UpdateCurrentJobCounts(running, queued)

	// Store a snapshot for historical charts
	if err := s.db.InsertMetricsSnapshot(running, queued); err != nil {
		logger.Logger.Error("Failed to insert metrics snapshot", zap.Error(err))
	}
}
