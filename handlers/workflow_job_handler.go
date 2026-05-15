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

	// Reject unknown statuses up-front: handing them to AddOrUpdateJob would
	// either silently no-op or, worse, treat them as the highest priority and
	// poison the row. This is a permanent error — replaying the same payload
	// will produce the same result, so do not spill it to webhook_events.
	if _, ok := models.JobStatusPriority(event.WorkflowJob.Status); !ok {
		logger.Logger.Warn("Unknown workflow_job action; refusing to upsert",
			zap.String("action", event.Action),
			zap.Int64("job_id", event.WorkflowJob.ID),
			zap.String("delivery_id", sequence.DeliveryID))
		return nil
	}

	// Get the previous state of this job from database to handle transitions correctly
	previousJob, err := h.db.GetWorkflowJobByID(context.TODO(), event.WorkflowJob.ID)
	if err != nil {
		logger.Logger.Error("Error getting previous job state",
			zap.Error(err),
			zap.Int64("job_id", event.WorkflowJob.ID))
		// Non-fatal: previousJob falls back to its zero value, so the
		// transition log line below will read previous_status as empty.
	}

	// Store job data in database with atomicity checks. AddOrUpdateJob applies
	// the lifecycle/terminal transition rules; a real error here means the
	// write itself failed and the caller should retry / spill, so we must NOT
	// swallow it.
	updated, err := h.db.AddOrUpdateJob(context.TODO(), event.WorkflowJob, sequence.Timestamp)
	if err != nil {
		logger.Logger.Error("Error saving job to database",
			zap.Error(err),
			zap.String("delivery_id", sequence.DeliveryID),
			zap.Int64("job_id", event.WorkflowJob.ID))
		return fmt.Errorf("failed to save workflow job: %w", err)
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

	logger.Logger.Debug("Event handled successfully", zap.String("event_type", h.GetEventType()))
	return nil
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

	prio, ok := models.JobStatusPriority(models.JobStatus(event.Action))
	if !ok {
		logger.Logger.Warn("Unknown job status", zap.String("status", event.Action))
		// Return 0 (lowest priority) — combined with the AddOrUpdateJob guard
		// this means an unknown action sorts to the start of an ordering
		// batch and is then rejected by the typed-table upsert.
		return 0, nil
	}
	return prio, nil
}
