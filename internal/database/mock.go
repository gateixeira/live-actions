package database

import (
	"context"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/stretchr/testify/mock"
)

type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) GetWorkflowRunsPaginated(ctx context.Context, page int, limit int) ([]models.WorkflowRun, int, error) {
	args := m.Called(ctx, page, limit)
	return args.Get(0).([]models.WorkflowRun), args.Int(1), args.Error(2)
}

func (m *MockDatabase) AddOrUpdateJob(ctx context.Context, workflowJob models.WorkflowJob, eventTimestamp time.Time) (bool, error) {
	args := m.Called(ctx, workflowJob, eventTimestamp)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabase) AddOrUpdateRun(ctx context.Context, workflowRun models.WorkflowRun, eventTimestamp time.Time) (bool, error) {
	args := m.Called(ctx, workflowRun, eventTimestamp)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabase) GetWorkflowJobsByRunID(ctx context.Context, runID int64) ([]models.WorkflowJob, error) {
	args := m.Called(ctx, runID)
	return args.Get(0).([]models.WorkflowJob), args.Error(1)
}

func (m *MockDatabase) CleanupOldData(ctx context.Context, retentionPeriod time.Duration) (int64, int64, int64, error) {
	args := m.Called(ctx, retentionPeriod)
	return args.Get(0).(int64), args.Get(1).(int64), args.Get(2).(int64), args.Error(3)
}

func (m *MockDatabase) GetWorkflowJobByID(ctx context.Context, jobID int64) (models.WorkflowJob, error) {
	args := m.Called(ctx, jobID)
	return args.Get(0).(models.WorkflowJob), args.Error(1)
}

func (m *MockDatabase) StoreWebhookEvent(ctx context.Context, event *models.OrderedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockDatabase) GetPendingEventsGrouped(ctx context.Context, limit int) ([]*models.OrderedEvent, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*models.OrderedEvent), args.Error(1)
}

func (m *MockDatabase) GetPendingEventsByAge(ctx context.Context, maxAge time.Duration, limit int) ([]*models.OrderedEvent, error) {
	args := m.Called(ctx, maxAge, limit)
	return args.Get(0).([]*models.OrderedEvent), args.Error(1)
}

func (m *MockDatabase) MarkEventProcessed(ctx context.Context, deliveryID string) error {
	args := m.Called(ctx, deliveryID)
	return args.Error(0)
}

func (m *MockDatabase) MarkEventFailed(ctx context.Context, deliveryID string) error {
	args := m.Called(ctx, deliveryID)
	return args.Error(0)
}

func (m *MockDatabase) GetCurrentJobCounts(ctx context.Context) (int, int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Int(1), args.Error(2)
}

func (m *MockDatabase) InsertMetricsSnapshot(ctx context.Context, running, queued int) error {
	args := m.Called(ctx, running, queued)
	return args.Error(0)
}

func (m *MockDatabase) GetMetricsHistory(ctx context.Context, since time.Duration) ([]models.MetricsSnapshot, error) {
	args := m.Called(ctx, since)
	return args.Get(0).([]models.MetricsSnapshot), args.Error(1)
}

func (m *MockDatabase) GetMetricsSummary(ctx context.Context, since time.Duration) (map[string]float64, error) {
	args := m.Called(ctx, since)
	return args.Get(0).(map[string]float64), args.Error(1)
}

func (m *MockDatabase) GetFailureAnalytics(ctx context.Context, since time.Duration) (*models.FailureAnalytics, error) {
	args := m.Called(ctx, since)
	return args.Get(0).(*models.FailureAnalytics), args.Error(1)
}

func (m *MockDatabase) GetFailureTrend(ctx context.Context, since time.Duration) ([]models.FailureTrendPoint, error) {
	args := m.Called(ctx, since)
	return args.Get(0).([]models.FailureTrendPoint), args.Error(1)
}
