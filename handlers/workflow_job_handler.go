package handlers

import (
	"context"
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

	// Get the previous state of this job from database to handle transitions correctly
	previousJob, err := h.db.GetWorkflowJobByID(context.TODO(), event.WorkflowJob.ID)
	if err != nil {
		logger.Logger.Error("Error getting previous job state",
			zap.Error(err),
			zap.Int64("job_id", event.WorkflowJob.ID))
		// Continue processing even if we can't get previous state
	}

	// Store job data in database with atomicity checks
	updated, err := h.db.AddOrUpdateJob(context.TODO(), event.WorkflowJob, sequence.Timestamp)
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
	// Query database for current job counts
	running, queued, err := h.db.GetCurrentJobCounts(context.TODO())
	if err != nil {
		logger.Logger.Error("Failed to query current job counts", zap.Error(err))
		return
	}

	// Convert to the expected format for SSE
	metricsUpdate := models.MetricsUpdateEvent{
		RunningJobs: running,
		QueuedJobs:  queued,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	logger.Logger.Debug("Sending metrics update",
		zap.Int("running_jobs", metricsUpdate.RunningJobs),
		zap.Int("queued_jobs", metricsUpdate.QueuedJobs))

	SendMetricsUpdate(metricsUpdate)
}

// handleJobStatusTransition manages state transitions correctly between job statuses
func (h *WorkflowJobHandler) handleJobStatusTransition(previousStatus, currentStatus models.JobStatus, job models.WorkflowJob) {
	metricsRegistry := metrics.GetRegistry()

	// Skip if status hasn't actually changed
	if previousStatus == currentStatus {
		logger.Logger.Debug("Job status unchanged, skipping metrics update",
			zap.Int64("job_id", job.ID),
			zap.String("status", string(currentStatus)))
		return
	}

	logger.Logger.Debug("Handling job status transition",
		zap.Int64("job_id", job.ID),
		zap.String("from", string(previousStatus)),
		zap.String("to", string(currentStatus)))

	label := "(unlabeled)"
	if len(job.Labels) > 0 {
		label = job.Labels[0]
	}

	// Record queue duration if transitioning from queued
	if previousStatus == models.JobStatusQueued && !job.StartedAt.IsZero() {
		queueTime := job.StartedAt.Sub(job.CreatedAt)
		metricsRegistry.RecordQueueDuration(label, queueTime.Seconds())
		logger.Logger.Debug("Queue time recorded",
			zap.Int64("job_id", job.ID),
			zap.Duration("queue_time", queueTime))
	}

	// Record conclusion when job completes
	if currentStatus == models.JobStatusCompleted && job.Conclusion != "" {
		metricsRegistry.RecordJobConclusion(job.Conclusion)
	}
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
