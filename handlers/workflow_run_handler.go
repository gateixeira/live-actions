package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"go.uber.org/zap"
)

type WorkflowRunHandler struct {
	db database.DatabaseInterface
}

func NewWorkflowRunHandler(db database.DatabaseInterface) *WorkflowRunHandler {
	return &WorkflowRunHandler{db: db}
}

func (h *WorkflowRunHandler) GetEventType() string {
	return "workflow_run"
}

func (h *WorkflowRunHandler) HandleEvent(eventData []byte, sequence *models.EventSequence) error {
	var event models.WorkflowRunEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		logger.Logger.Error("Failed to parse workflow_run JSON payload",
			zap.Error(err),
			zap.String("delivery_id", sequence.DeliveryID),
			zap.String("event_id", sequence.EventID))
		logger.Logger.Debug("Raw event data",
			zap.ByteString("data", eventData),
			zap.String("delivery_id", sequence.DeliveryID))
		return fmt.Errorf("invalid JSON payload: %w", err)
	}

	event.WorkflowRun.Status = models.JobStatus(event.Action)
	event.WorkflowRun.RepositoryName = event.Repository.Name

	// Reject unknown statuses up-front (see WorkflowJobHandler.HandleEvent for
	// the rationale). Permanent error — do not spill / retry.
	if _, ok := models.RunStatusPriority(event.WorkflowRun.Status); !ok {
		logger.Logger.Warn("Unknown workflow_run action; refusing to upsert",
			zap.String("action", event.Action),
			zap.Int64("run_id", event.WorkflowRun.ID),
			zap.String("delivery_id", sequence.DeliveryID))
		return nil
	}

	logger.Logger.Info("Processing workflow run event",
		zap.String("action", event.Action),
		zap.Int64("run_id", event.WorkflowRun.ID),
		zap.String("repository", event.WorkflowRun.RepositoryName),
		zap.String("delivery_id", sequence.DeliveryID),
		zap.Time("event_timestamp", sequence.Timestamp),
		zap.Time("received_at", sequence.ReceivedAt))

	// Store run data in database with atomicity checks
	updated, err := h.db.AddOrUpdateRun(context.TODO(), event.WorkflowRun, sequence.Timestamp)
	if err != nil {
		logger.Logger.Error("Error saving run to database",
			zap.Error(err),
			zap.String("delivery_id", sequence.DeliveryID),
			zap.Int64("run_id", event.WorkflowRun.ID))
		return fmt.Errorf("failed to save workflow run: %w", err)
	}

	// If the run was not updated due to atomicity constraints, skip further processing
	if !updated {
		logger.Logger.Info("Skipping older event for run that already reached terminal state",
			zap.Int64("run_id", event.WorkflowRun.ID),
			zap.String("incoming_status", string(event.WorkflowRun.Status)),
			zap.Time("event_timestamp", sequence.Timestamp),
			zap.String("delivery_id", sequence.DeliveryID))
		return nil
	}

	// Send SSE event for workflow run update
	SendWorkflowUpdate(models.WorkflowUpdateEvent{
		Type:        "run",
		Action:      event.Action,
		ID:          event.WorkflowRun.ID,
		Status:      string(event.WorkflowRun.Status),
		Timestamp:   time.Now().Format(time.RFC3339),
		WorkflowRun: event.WorkflowRun,
	})

	logger.Logger.Debug("Event handled successfully", zap.String("event_type", h.GetEventType()))
	return nil
}

func (h *WorkflowRunHandler) ExtractEventTimestamp(eventData []byte) (time.Time, error) {
	var event models.WorkflowRunEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse workflow_run JSON payload: %w", err)
	}

	return event.WorkflowRun.CreatedAt, nil
}

func (h *WorkflowRunHandler) ExtractOrderingKey(eventData []byte) (string, error) {
	var event models.WorkflowRunEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return "", fmt.Errorf("failed to parse workflow_run JSON payload: %w", err)
	}

	return fmt.Sprintf("run_%d", event.WorkflowRun.ID), nil
}

func (h *WorkflowRunHandler) GetStatusPriority(eventData []byte) (int, error) {
	var event models.WorkflowRunEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return 0, fmt.Errorf("failed to parse workflow_run JSON payload: %w", err)
	}

	prio, ok := models.RunStatusPriority(models.JobStatus(event.Action))
	if !ok {
		logger.Logger.Warn("Unknown run status", zap.String("status", event.Action))
		return 0, nil
	}
	return prio, nil
}
