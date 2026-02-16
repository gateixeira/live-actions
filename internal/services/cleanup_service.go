package services

import (
	"context"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/pkg/logger"
	"go.uber.org/zap"
)

// CleanupService handles automatic cleanup of old data
type CleanupService struct {
	config *config.Config
	db     database.DatabaseInterface
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewCleanupService creates a new cleanup service instance
func NewCleanupService(config *config.Config, db database.DatabaseInterface, ctx context.Context) *CleanupService {
	ctx, cancel := context.WithCancel(ctx)

	return &CleanupService{
		config: config,
		db:     db,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

// Start begins the cleanup service with periodic data cleanup
func (cs *CleanupService) Start() {
	defer close(cs.done)

	ticker := time.NewTicker(cs.config.GetCleanupInterval())
	defer ticker.Stop()

	// Perform initial cleanup on startup
	if err := cs.performCleanup(); err != nil {
		logger.Logger.Error("Initial cleanup failed", zap.Error(err))
	}

	for {
		select {
		case <-cs.ctx.Done():
			logger.Logger.Debug("Cleanup service stopped")
			return
		case <-ticker.C:
			logger.Logger.Info("Cleanup service started",
				zap.Duration("retention_period", cs.config.GetDataRetentionDuration()),
				zap.Duration("cleanup_interval", cs.config.GetCleanupInterval()),
			)

			if err := cs.performCleanup(); err != nil {
				logger.Logger.Error("Scheduled cleanup failed", zap.Error(err))
			}
		}
	}
}

// Stop gracefully stops the cleanup service
func (cs *CleanupService) Stop() {
	cs.cancel()
	<-cs.done
}

// performCleanup executes the actual cleanup operation
func (cs *CleanupService) performCleanup() error {
	retentionPeriod := cs.config.GetDataRetentionDuration()

	logger.Logger.Debug("Starting data cleanup",
		zap.Duration("retention_period", retentionPeriod),
		zap.Time("cutoff_time", time.Now().Add(-retentionPeriod)),
	)

	deletedRuns, deletedJobs, deletedEvents, err := cs.db.CleanupOldData(cs.ctx, retentionPeriod)
	if err != nil {
		logger.Logger.Error("Data cleanup failed", zap.Error(err))
		return err
	}

	if deletedRuns > 0 || deletedJobs > 0 {
		logger.Logger.Info("Data cleanup completed",
			zap.Int64("deleted_workflow_runs", deletedRuns),
			zap.Int64("deleted_workflow_jobs", deletedJobs),
			zap.Int64("deleted_webhook_events", deletedEvents),
			zap.Duration("retention_period", retentionPeriod),
		)
	} else {
		logger.Logger.Debug("Data cleanup completed - no old data found",
			zap.Duration("retention_period", retentionPeriod),
		)
	}

	return nil
}
