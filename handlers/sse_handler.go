package handlers

import (
	"encoding/json"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SSEEvent represents a server-sent event
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// SSEHandler handles server-sent events
type SSEHandler struct {
	client chan SSEEvent
}

// Global SSE handler instance
var sseHandler *SSEHandler

func InitSSEHandler() {
	sseHandler = &SSEHandler{
		client: make(chan SSEEvent, 100),
	}
}

func GetSSEHandler() *SSEHandler {
	return sseHandler
}

func (h *SSEHandler) SendEvent(eventType string, data interface{}) {
	if h == nil || h.client == nil {
		return
	}

	event := SSEEvent{
		Type: eventType,
		Data: data,
	}

	select {
	case h.client <- event:
		logger.Logger.Debug("SSE event sent", zap.String("type", eventType))
	default:
		logger.Logger.Debug("SSE channel full, dropping event", zap.String("type", eventType))
	}
}

func (h *SSEHandler) HandleSSE() gin.HandlerFunc {
	return func(c *gin.Context) {

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Cache-Control")

		clientChan := make(chan SSEEvent, 100)

		go func() {
			for {
				select {
				case event := <-h.client:
					select {
					case clientChan <- event:
					default:
						// Client channel full, skip this event
					}
				case <-c.Request.Context().Done():
					// Client disconnected
					close(clientChan)
					return
				}
			}
		}()

		// Send initial connection event
		c.SSEvent("message", map[string]interface{}{
			"type": "connected",
			"data": map[string]string{
				"message":   "SSE connection established",
				"timestamp": time.Now().Format(time.RFC3339),
			},
		})

		// Keep connection alive and send events
		for {
			select {
			case event, ok := <-clientChan:
				if !ok {
					// Channel closed, client disconnected
					return
				}

				jsonData, err := json.Marshal(event)
				if err != nil {
					logger.Logger.Error("Failed to marshal SSE event", zap.Error(err))
					continue
				}

				c.SSEvent("message", string(jsonData))
				c.Writer.Flush()

			case <-c.Request.Context().Done():
				// Client disconnected
				logger.Logger.Debug("SSE client disconnected")
				return

			case <-time.After(30 * time.Second):
				// Send keepalive ping
				c.SSEvent("ping", map[string]string{
					"timestamp": time.Now().Format(time.RFC3339),
				})
				c.Writer.Flush()
			}
		}
	}
}

// SendMetricsUpdate sends a metrics update event
func SendMetricsUpdate(update models.MetricsUpdateEvent) {
	if sseHandler != nil {
		sseHandler.SendEvent("metrics_update", update)
	}
}

// SendWorkflowUpdate sends a workflow update event
func SendWorkflowUpdate(update models.WorkflowUpdateEvent) {
	if sseHandler != nil {
		sseHandler.SendEvent("workflow_update", update)
	}
}
