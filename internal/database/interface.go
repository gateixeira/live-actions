package database

import (
	"time"

	"github.com/gateixeira/live-actions/models"
)

// DatabaseInterface defines the contract for database operations
type DatabaseInterface interface {
	// Workflow Jobs
	AddOrUpdateJob(workflowJob models.WorkflowJob, eventTimestamp time.Time) (bool, error)
	GetWorkflowJobByID(jobID int64) (models.WorkflowJob, error)
	GetJobsByLabel(page int, limit int) ([]models.LabelMetrics, int, error)
	GetWorkflowJobsByRunID(runID int64) ([]models.WorkflowJob, error)
	GetCurrentJobCounts() (map[string]map[string]int, error)

	// Workflow Runs
	AddOrUpdateRun(workflowRun models.WorkflowRun, eventTimestamp time.Time) (bool, error)
	GetWorkflowRunsPaginated(page int, limit int) ([]models.WorkflowRun, int, error)

	// Metrics Snapshots
	InsertMetricsSnapshot(running, queued int) error
	GetMetricsHistory(since time.Duration) ([]models.MetricsSnapshot, error)
	GetMetricsSummary(since time.Duration) (map[string]float64, error)

	// Webhook Events
	StoreWebhookEvent(event *models.OrderedEvent) error
	GetPendingEventsGrouped(limit int) ([]*models.OrderedEvent, error)
	GetPendingEventsByAge(maxAge time.Duration, limit int) ([]*models.OrderedEvent, error)
	MarkEventProcessed(deliveryID string) error
	MarkEventFailed(deliveryID string) error

	// Cleanup
	CleanupOldData(retentionPeriod time.Duration) (int64, int64, int64, error)
}

// DBWrapper wraps the actual DB instance and implements DatabaseInterface
type DBWrapper struct{}

// NewDBWrapper creates a new DBWrapper instance
func NewDBWrapper() DatabaseInterface {
	return &DBWrapper{}
}
