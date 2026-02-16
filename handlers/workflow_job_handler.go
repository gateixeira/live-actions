package handlers

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gateixeira/live-actions/pkg/metrics"
	"go.uber.org/zap"
)

type WorkflowJobHandler struct {
	mutex  sync.RWMutex
	db     database.DatabaseInterface
	config *config.Config
}

func NewWorkflowJobHandler(config *config.Config, db database.DatabaseInterface) *WorkflowJobHandler {
	return &WorkflowJobHandler{
		db:     db,
		config: config,
	}
}

func (h *WorkflowJobHandler) GetEventType() string {
	return "workflow_job"
}

func (h *WorkflowJobHandler) HandleEvent(eventData []byte, sequence *models.EventSequence) error {
	var event models.WorkflowJobEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		logger.Logger.Error("Failed to parse workflow_job JSON payload",
			zap.Error(err),
			zap.String("delivery_id", sequence.DeliveryID),
			zap.String("event_id", sequence.EventID))
		return fmt.Errorf("invalid JSON payload: %w", err)
	}

	event.WorkflowJob.Status = models.JobStatus(event.Action)
	event.WorkflowJob.RunnerType = h.inferRunnerType(event.WorkflowJob.Labels)

	// Get the previous state of this job from database to handle transitions correctly
	previousJob, err := h.db.GetWorkflowJobByID(event.WorkflowJob.ID)
	if err != nil {
		logger.Logger.Error("Error getting previous job state",
			zap.Error(err),
			zap.Int64("job_id", event.WorkflowJob.ID))
		// Continue processing even if we can't get previous state
	}

	// Store job data in database with atomicity checks
	updated, err := h.db.AddOrUpdateJob(event.WorkflowJob, sequence.Timestamp)
	if err != nil {
		logger.Logger.Error("Error saving job to database",
			zap.Error(err),
			zap.String("delivery_id", sequence.DeliveryID),
			zap.Int64("job_id", event.WorkflowJob.ID))
		// Continue processing even if database save fails
	}

	// If the job was not updated due to atomicity constraints, skip further processing
	if !updated {
		logger.Logger.Info("Skipping older event for job that already reached terminal state",
			zap.Int64("job_id", event.WorkflowJob.ID),
			zap.String("incoming_status", string(event.WorkflowJob.Status)),
			zap.Time("event_timestamp", sequence.Timestamp),
			zap.String("delivery_id", sequence.DeliveryID))
		return nil
	}

	h.mutex.Lock()
	defer h.mutex.Unlock()

	logger.Logger.Info("Processing workflow job event",
		zap.String("action", event.Action),
		zap.Int64("job_id", event.WorkflowJob.ID),
		zap.String("current_status", string(event.WorkflowJob.Status)),
		zap.String("previous_status", string(previousJob.Status)),
		zap.String("delivery_id", sequence.DeliveryID),
		zap.Time("event_timestamp", sequence.Timestamp),
		zap.Time("received_at", sequence.ReceivedAt))

	// Handle state transitions correctly
	h.handleJobStatusTransition(previousJob.Status, event.WorkflowJob.Status, event.WorkflowJob)

	h.sendMetricsUpdate()

	logger.Logger.Debug("Event handled successfully", zap.String("event_type", h.GetEventType()))
	return nil
}

func (h *WorkflowJobHandler) sendMetricsUpdate() {

	// Query database for current job counts directly (same source as label metrics)
	jobCounts, err := h.db.GetCurrentJobCounts()
	if err != nil {
		logger.Logger.Error("Failed to query current job counts", zap.Error(err))
		return
	}

	// Calculate totals from database data
	var runningTotal, queuedTotal int
	for _, statusCounts := range jobCounts {
		runningTotal += statusCounts["running"]
		queuedTotal += statusCounts["queued"]
	}

	// Fetch label metrics from database
	labelMetrics, _, err := h.db.GetJobsByLabel(1, 10) // Get first 10 label metrics
	if err != nil {
		logger.Logger.Error("Failed to query label metrics", zap.Error(err))
		// Continue without label metrics rather than failing the entire update
		labelMetrics = []models.LabelMetrics{}
	}

	// Convert to the expected format for SSE
	metrics := models.MetricsUpdateEvent{
		RunningJobs:  runningTotal,
		QueuedJobs:   queuedTotal,
		Timestamp:    time.Now().Format(time.RFC3339),
		LabelMetrics: labelMetrics,
	}

	logger.Logger.Debug("Sending metrics update",
		zap.Int("running_jobs", metrics.RunningJobs),
		zap.Int("queued_jobs", metrics.QueuedJobs),
		zap.Int("label_metrics_count", len(metrics.LabelMetrics)))

	SendMetricsUpdate(metrics)
}

// handleJobStatusTransition manages state transitions correctly between job statuses
func (h *WorkflowJobHandler) handleJobStatusTransition(previousStatus, currentStatus models.JobStatus, job models.WorkflowJob) {
	metricsRegistry := metrics.GetRegistry()
	runnerType := string(job.RunnerType)

	// Skip if status hasn't actually changed
	if previousStatus == currentStatus {
		logger.Logger.Debug("Job status unchanged, skipping metrics update",
			zap.Int64("job_id", job.ID),
			zap.String("status", string(currentStatus)),
			zap.String("runner_type", runnerType))
		return
	}

	logger.Logger.Debug("Handling job status transition",
		zap.Int64("job_id", job.ID),
		zap.String("from", string(previousStatus)),
		zap.String("to", string(currentStatus)),
		zap.String("runner_type", runnerType))

	// Record queue duration if transitioning from queued
	if previousStatus == models.JobStatusQueued && !job.StartedAt.IsZero() {
		queueTime := job.StartedAt.Sub(job.CreatedAt)
		metricsRegistry.RecordQueueDuration(job.RunnerType, queueTime.Seconds())
		logger.Logger.Debug("Queue time recorded",
			zap.Int64("job_id", job.ID),
			zap.Duration("queue_time", queueTime),
			zap.String("runner_type", runnerType))
	}

	logger.Logger.Debug("Job status transition recorded for database-driven metrics",
		zap.String("from_status", string(previousStatus)),
		zap.String("to_status", string(currentStatus)),
		zap.String("runner_type", string(runnerType)))
}

func (h *WorkflowJobHandler) inferRunnerType(labels []string) models.RunnerType {
	return h.config.RunnerTypeConfig.InferRunnerType(labels)
}

func (h *WorkflowJobHandler) ExtractEventTimestamp(eventData []byte) (time.Time, error) {
	var event models.WorkflowJobEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse workflow_job JSON payload: %w", err)
	}

	return event.WorkflowJob.CreatedAt, nil
}

func (h *WorkflowJobHandler) ExtractOrderingKey(eventData []byte) (string, error) {
	var event models.WorkflowJobEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return "", fmt.Errorf("failed to parse workflow_job JSON payload: %w", err)
	}

	return fmt.Sprintf("job_%d", event.WorkflowJob.ID), nil
}

func (h *WorkflowJobHandler) GetStatusPriority(eventData []byte) (int, error) {
	var event models.WorkflowJobEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return 0, fmt.Errorf("failed to parse workflow_job JSON payload: %w", err)
	}

	switch models.JobStatus(event.Action) {
	case models.JobStatusWaiting:
		return 1, nil
	case models.JobStatusQueued:
		return 2, nil
	case models.JobStatusRequested:
		return 3, nil
	case models.JobStatusInProgress:
		return 4, nil
	case models.JobStatusCompleted, models.JobStatusCancelled:
		return 5, nil
	default:
		logger.Logger.Warn("Unknown job status", zap.String("status", event.Action))
		return 999, nil
	}
}
