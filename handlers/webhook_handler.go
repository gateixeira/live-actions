package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/services"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gateixeira/live-actions/pkg/metrics"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	GitHubSignatureHeader = "X-Hub-Signature-256"
	GitHubEventHeader     = "X-GitHub-Event"
	GitHubDeliveryHeader  = "X-GitHub-Delivery"
)

// ValidateGitHubWebhook middleware validates the GitHub webhook signature and event type
func ValidateGitHubWebhook(config *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		webhookSecret := config.Vars.WebhookSecret
		if webhookSecret == "" {
			logger.Logger.Error("WEBHOOK_SECRET is not configured, rejecting webhook")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Webhook secret not configured"})
			c.Abort()
			return
		}

		signature := c.GetHeader(GitHubSignatureHeader)
		if signature == "" {
			logger.Logger.Error("Webhook validation failed: Missing X-Hub-Signature-256 header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing signature header"})
			c.Abort()
			return
		}

		signatureHash := signature
		if len(signature) > 7 && signature[0:7] == "sha256=" {
			signatureHash = signature[7:]
		}

		// Limit request body size to prevent memory exhaustion (10 MB)
		const maxBodySize = 10 * 1024 * 1024
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodySize)

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Logger.Error("Error reading request body", zap.Error(err))
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Request body too large"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
			}
			c.Abort()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		mac := hmac.New(sha256.New, []byte(webhookSecret))
		mac.Write(body)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		expectedBytes, err := hex.DecodeString(expectedSignature)
		if err != nil {
			logger.Logger.Error("Error decoding expected signature", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate signature"})
			c.Abort()
			return
		}

		receivedBytes, err := hex.DecodeString(signatureHash)
		if err != nil {
			logger.Logger.Error("Error decoding received signature", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature format"})
			c.Abort()
			return
		}

		if !hmac.Equal(expectedBytes, receivedBytes) {
			logger.Logger.Error("Webhook validation failed: Invalid signature")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			c.Abort()
			return
		}

		eventType := c.GetHeader(GitHubEventHeader)
		if eventType == "" {
			logger.Logger.Error("Missing event type header")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing event type"})
			c.Abort()
			return
		}

		// Store event type in context for the handler
		c.Set("eventType", eventType)
		c.Next()
	}
}

// IngestResult is the structured outcome of accepting one webhook delivery,
// independent of the transport (HTTP POST or WebSocket relay frame).
// `Status` mirrors the HTTP status code semantics so a transport can map
// it directly into its own response (HTTP status code, WS ack frame, etc.).
//   - 202: accepted and enqueued for processing
//   - 200: ignored (no handler registered for this event type)
//   - 400: payload rejected (bad JSON, missing fields, unknown status)
//   - 503: ingest queue saturated; the caller should retry the delivery
//   - 500: internal failure not classified above
type IngestResult struct {
	Status  int
	Message string
}

// Ingest performs the transport-agnostic part of accepting a webhook
// delivery: payload decoding, validation, extraction of ordering metadata
// and enqueue onto the ordering service. Signature verification is the
// caller's responsibility (HTTP middleware verifies HMAC; the WebSocket
// relay is authenticated by token at connection time and does not need
// per-frame verification).
func (h *WebhookHandler) Ingest(eventType, deliveryID string, body []byte) IngestResult {
	if eventType == "" {
		return IngestResult{Status: http.StatusBadRequest, Message: "Missing event type"}
	}
	if deliveryID == "" {
		return IngestResult{Status: http.StatusBadRequest, Message: "Missing delivery ID"}
	}

	// GitHub may send either application/json or application/x-www-form-urlencoded;
	// only the form variant needs payload= unwrapping.
	jsonData := body
	if bodyStr := string(body); strings.HasPrefix(bodyStr, "payload=") {
		decodedBody, err := url.QueryUnescape(bodyStr)
		if err != nil {
			logger.Logger.Error("Failed to decode URL-encoded payload", zap.Error(err))
			return IngestResult{Status: http.StatusBadRequest, Message: "Failed to decode URL-encoded payload"}
		}
		const prefix = "payload="
		if !strings.HasPrefix(decodedBody, prefix) {
			logger.Logger.Error("URL-encoded payload does not start with expected prefix",
				zap.String("expected_prefix", prefix),
				zap.String("payload_start", decodedBody[:min(len(decodedBody), 50)]))
			return IngestResult{Status: http.StatusBadRequest, Message: "Invalid URL-encoded payload format"}
		}
		jsonData = []byte(decodedBody[len(prefix):])
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(jsonData, &payload); err != nil {
		logger.Logger.Error("Failed to parse JSON payload",
			zap.Error(err),
			zap.String("payload_start", string(jsonData[:min(len(jsonData), 100)])))
		return IngestResult{Status: http.StatusBadRequest, Message: "Invalid JSON payload"}
	}

	handler, exists := h.handlers[eventType]
	if !exists {
		logger.Logger.Warn("No handler registered for event type", zap.String("event_type", eventType))
		metrics.GetRegistry().WebhookEventsTotal.WithLabelValues(eventType, "ignored").Inc()
		return IngestResult{Status: http.StatusOK, Message: "Event type not supported"}
	}

	extractedTime, err := handler.ExtractEventTimestamp(jsonData)
	if err != nil {
		logger.Logger.Error("Failed to extract event timestamp",
			zap.Error(err),
			zap.String("event_type", eventType),
			zap.String("delivery_id", deliveryID))
		metrics.GetRegistry().WebhookEventsTotal.WithLabelValues(eventType, "rejected_invalid").Inc()
		return IngestResult{Status: http.StatusBadRequest, Message: "Failed to extract event timestamp"}
	}

	orderingKey, err := handler.ExtractOrderingKey(jsonData)
	if err != nil {
		logger.Logger.Error("Failed to extract ordering key",
			zap.Error(err),
			zap.String("event_type", eventType),
			zap.String("delivery_id", deliveryID))
		metrics.GetRegistry().WebhookEventsTotal.WithLabelValues(eventType, "rejected_invalid").Inc()
		return IngestResult{Status: http.StatusBadRequest, Message: "Failed to extract ordering key"}
	}

	statusPriority, err := handler.GetStatusPriority(jsonData)
	if err != nil {
		logger.Logger.Error("Failed to extract status priority",
			zap.Error(err),
			zap.String("event_type", eventType),
			zap.String("delivery_id", deliveryID))
		metrics.GetRegistry().WebhookEventsTotal.WithLabelValues(eventType, "rejected_invalid").Inc()
		return IngestResult{Status: http.StatusBadRequest, Message: "Failed to extract status priority"}
	}

	orderedEvent := &models.OrderedEvent{
		Sequence: models.EventSequence{
			EventID:    deliveryID,
			Timestamp:  extractedTime,
			DeliveryID: deliveryID,
			ReceivedAt: time.Now(),
		},
		EventType:      eventType,
		RawPayload:     jsonData,
		OrderingKey:    orderingKey,
		StatusPriority: statusPriority,
	}

	if err := h.orderingService.AddEvent(orderedEvent); err != nil {
		if errors.Is(err, services.ErrIngestQueueFull) {
			logger.Logger.Error("Webhook ingest queue full; rejecting delivery",
				zap.String("delivery_id", deliveryID),
				zap.String("event_type", eventType))
			metrics.GetRegistry().WebhookEventsTotal.WithLabelValues(eventType, "rejected_queue_full").Inc()
			return IngestResult{Status: http.StatusServiceUnavailable, Message: "Server overloaded; manual redelivery required"}
		}
		logger.Logger.Error("Failed to add event to ordering service", zap.Error(err))
		metrics.GetRegistry().WebhookEventsTotal.WithLabelValues(eventType, "rejected_invalid").Inc()
		return IngestResult{Status: http.StatusInternalServerError, Message: "Failed to process event"}
	}

	metrics.GetRegistry().WebhookEventsTotal.WithLabelValues(eventType, "accepted").Inc()

	logger.Logger.Debug("Event queued for ordered processing",
		zap.String("event_type", orderedEvent.EventType),
		zap.String("delivery_id", orderedEvent.Sequence.DeliveryID),
		zap.String("ordering_key", orderedEvent.OrderingKey),
		zap.Int("status_priority", orderedEvent.StatusPriority),
	)

	return IngestResult{Status: http.StatusAccepted, Message: "Event queued for processing"}
}

// Handle processes incoming webhook events
func (h *WebhookHandler) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Logger.Error("Failed to read request body", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}

		eventTypeVal, exists := c.Get("eventType")
		if !exists {
			logger.Logger.Error("Event type not found in context")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing event type"})
			return
		}
		eventTypeStr, ok := eventTypeVal.(string)
		if !ok {
			logger.Logger.Error("Event type is not a string")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event type"})
			return
		}

		deliveryID := c.GetHeader(GitHubDeliveryHeader)

		result := h.Ingest(eventTypeStr, deliveryID, body)
		switch {
		case result.Status == http.StatusAccepted:
			c.JSON(result.Status, gin.H{"status": "queued", "message": result.Message})
		case result.Status == http.StatusOK:
			c.JSON(result.Status, gin.H{"status": "ignored", "message": result.Message})
		default:
			c.JSON(result.Status, gin.H{"error": result.Message})
		}
	}
}

func (h *WebhookHandler) processOrderedEvent(event *models.OrderedEvent) error {
	// On the happy path the event arrives straight from the in-memory ingest
	// channel and was never written to webhook_events; only spilled events
	// (event.Persisted == true) need MarkEventProcessed/MarkEventFailed.

	handler, exists := h.handlers[event.EventType]

	if !exists {
		logger.Logger.Warn("No handler registered for event type", zap.String("event_type", event.EventType))
		return fmt.Errorf("event type %s: %w", event.EventType, services.ErrPermanent)
	}

	jsonData := event.RawPayload

	err := handler.HandleEvent(jsonData, &event.Sequence)
	if err != nil {
		logger.Logger.Error("Failed to handle event", zap.Error(err),
			zap.String("event_type", event.EventType),
			zap.String("delivery_id", event.Sequence.DeliveryID))
		if event.Persisted {
			_ = h.db.MarkEventFailed(context.TODO(), event.Sequence.DeliveryID)
		}
		return fmt.Errorf("failed to handle event: %w", err)
	}

	if event.Persisted {
		return h.db.MarkEventProcessed(context.TODO(), event.Sequence.DeliveryID)
	}
	return nil
}

func (h *WebhookHandler) Shutdown() {
	if h.orderingService != nil {
		h.orderingService.Stop()
		logger.Logger.Info("WebhookHandler ordering service stopped")
	}
}
