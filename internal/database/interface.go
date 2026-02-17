package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// DatabaseInterface defines the contract for database operations
type DatabaseInterface interface {
	// Workflow Jobs
	AddOrUpdateJob(ctx context.Context, workflowJob models.WorkflowJob, eventTimestamp time.Time) (bool, error)
	GetWorkflowJobByID(ctx context.Context, jobID int64) (models.WorkflowJob, error)
	GetWorkflowJobsByRunID(ctx context.Context, runID int64) ([]models.WorkflowJob, error)
	GetCurrentJobCounts(ctx context.Context) (int, int, error)

	// Workflow Runs
	AddOrUpdateRun(ctx context.Context, workflowRun models.WorkflowRun, eventTimestamp time.Time) (bool, error)
	GetWorkflowRunsPaginated(ctx context.Context, page int, limit int) ([]models.WorkflowRun, int, error)

	// Metrics Snapshots
	InsertMetricsSnapshot(ctx context.Context, running, queued int) error
	GetMetricsHistory(ctx context.Context, since time.Duration) ([]models.MetricsSnapshot, error)
	GetMetricsSummary(ctx context.Context, since time.Duration) (map[string]float64, error)

	// Webhook Events
	StoreWebhookEvent(ctx context.Context, event *models.OrderedEvent) error
	GetPendingEventsGrouped(ctx context.Context, limit int) ([]*models.OrderedEvent, error)
	GetPendingEventsByAge(ctx context.Context, maxAge time.Duration, limit int) ([]*models.OrderedEvent, error)
	MarkEventProcessed(ctx context.Context, deliveryID string) error
	MarkEventFailed(ctx context.Context, deliveryID string) error

	// Cleanup
	CleanupOldData(ctx context.Context, retentionPeriod time.Duration) (int64, int64, int64, error)

	// Failure Analytics
	GetFailureAnalytics(ctx context.Context, since time.Duration) (*models.FailureAnalytics, error)
	GetFailureTrend(ctx context.Context, since time.Duration) ([]models.FailureTrendPoint, error)
}

// DBWrapper wraps the actual DB instance and implements DatabaseInterface
type DBWrapper struct {
	db *sql.DB
}

// NewDBWrapper creates a new DBWrapper instance
func NewDBWrapper(db *sql.DB) DatabaseInterface {
	return &DBWrapper{db: db}
}
