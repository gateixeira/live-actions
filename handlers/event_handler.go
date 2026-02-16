package handlers

import (
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/internal/services"
	"github.com/gateixeira/live-actions/models"
)

type EventHandler interface {
	HandleEvent(eventData []byte, sequence *models.EventSequence) error
	GetEventType() string
	ExtractEventTimestamp(eventData []byte) (time.Time, error)
	ExtractOrderingKey(eventData []byte) (string, error)
	GetStatusPriority(eventData []byte) (int, error)
}

type WebhookHandler struct {
	db              database.DatabaseInterface
	handlers        map[string]EventHandler
	orderingService *services.EventOrderingService
}

func NewWebhookHandler(config *config.Config, db database.DatabaseInterface) *WebhookHandler {
	wh := &WebhookHandler{
		db:       db,
		handlers: make(map[string]EventHandler),
	}

	wh.orderingService = services.NewEventOrderingService(db, wh.processOrderedEvent)
	wh.orderingService.Start()

	wh.RegisterHandler(NewWorkflowJobHandler(config, db))
	wh.RegisterHandler(NewWorkflowRunHandler(db))

	return wh
}

func (h *WebhookHandler) RegisterHandler(handler EventHandler) {
	h.handlers[handler.GetEventType()] = handler
}
