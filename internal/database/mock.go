package database

import (
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/stretchr/testify/mock"
)

type MockDatabase struct {
	mock.Mock
}

// GetJobsByLabel implements DatabaseInterface.
func (m *MockDatabase) GetJobsByLabel(page int, limit int) ([]models.LabelMetrics, int, error) {
	args := m.Called(page, limit)
	return args.Get(0).([]models.LabelMetrics), args.Int(1), args.Error(2)
}

func (m *MockDatabase) GetWorkflowRunsPaginated(page int, limit int) ([]models.WorkflowRun, int, error) {
	args := m.Called(page, limit)
	return args.Get(0).([]models.WorkflowRun), args.Int(1), args.Error(2)
}

func (m *MockDatabase) AddOrUpdateJob(workflowJob models.WorkflowJob, eventTimestamp time.Time) (bool, error) {
	args := m.Called(workflowJob, eventTimestamp)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabase) AddOrUpdateRun(workflowRun models.WorkflowRun, eventTimestamp time.Time) (bool, error) {
	args := m.Called(workflowRun, eventTimestamp)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabase) GetWorkflowJobsByRunID(runID int64) ([]models.WorkflowJob, error) {
	args := m.Called(runID)
	return args.Get(0).([]models.WorkflowJob), args.Error(1)
}

func (m *MockDatabase) CleanupOldData(retentionPeriod time.Duration) (int64, int64, int64, error) {
	args := m.Called(retentionPeriod)
	return args.Get(0).(int64), args.Get(1).(int64), args.Get(2).(int64), args.Error(3)
}

func (m *MockDatabase) GetWorkflowJobByID(jobID int64) (models.WorkflowJob, error) {
	args := m.Called(jobID)
	return args.Get(0).(models.WorkflowJob), args.Error(1)
}

func (m *MockDatabase) StoreWebhookEvent(event *models.OrderedEvent) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockDatabase) GetPendingEventsGrouped(limit int) ([]*models.OrderedEvent, error) {
	args := m.Called(limit)
	return args.Get(0).([]*models.OrderedEvent), args.Error(1)
}

func (m *MockDatabase) GetPendingEventsByAge(maxAge time.Duration, limit int) ([]*models.OrderedEvent, error) {
	args := m.Called(maxAge, limit)
	return args.Get(0).([]*models.OrderedEvent), args.Error(1)
}

func (m *MockDatabase) MarkEventProcessed(deliveryID string) error {
	args := m.Called(deliveryID)
	return args.Error(0)
}

func (m *MockDatabase) MarkEventFailed(deliveryID string) error {
	args := m.Called(deliveryID)
	return args.Error(0)
}

func (m *MockDatabase) GetCurrentJobCounts() (map[string]map[string]int, error) {
	args := m.Called()
	return args.Get(0).(map[string]map[string]int), args.Error(1)
}

func (m *MockDatabase) InsertMetricsSnapshot(running, queued int) error {
	args := m.Called(running, queued)
	return args.Error(0)
}

func (m *MockDatabase) GetMetricsHistory(since time.Duration) ([]models.MetricsSnapshot, error) {
	args := m.Called(since)
	return args.Get(0).([]models.MetricsSnapshot), args.Error(1)
}

func (m *MockDatabase) GetMetricsSummary(since time.Duration) (map[string]float64, error) {
	args := m.Called(since)
	return args.Get(0).(map[string]float64), args.Error(1)
}
