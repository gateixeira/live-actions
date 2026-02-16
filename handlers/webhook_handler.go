package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
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

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Logger.Error("Error reading request body", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
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

// Handle processes incoming webhook events
func (h *WebhookHandler) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Logger.Error("Failed to read request body", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}

		// Parse event type from context
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
		if deliveryID == "" {
			logger.Logger.Error("Missing X-GitHub-Delivery header")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing delivery ID"})
			return
		}

		// Handle different payload formats
		var jsonData []byte
		bodyStr := string(body)

		// Check if this is a URL-encoded payload
		if strings.HasPrefix(bodyStr, "payload=") {
			// URL-encoded payload - extract the JSON part
			decodedBody, err := url.QueryUnescape(bodyStr)
			if err != nil {
				logger.Logger.Error("Failed to decode URL-encoded payload", zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decode URL-encoded payload"})
				return
			}

			const prefix = "payload="
			if !strings.HasPrefix(decodedBody, prefix) {
				logger.Logger.Error("URL-encoded payload does not start with expected prefix",
					zap.String("expected_prefix", prefix),
					zap.String("payload_start", decodedBody[:min(len(decodedBody), 50)]))
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL-encoded payload format"})
				return
			}
			jsonData = []byte(decodedBody[len(prefix):])
		} else {
			// Direct JSON payload
			jsonData = body
		}

		// Validate that we have valid JSON
		var payload map[string]interface{}
		if err := json.Unmarshal(jsonData, &payload); err != nil {
			logger.Logger.Error("Failed to parse JSON payload",
				zap.Error(err),
				zap.String("payload_start", string(jsonData[:min(len(jsonData), 100)])))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
			return
		}

		handler := h.handlers[eventTypeStr]
		extractedTime, err := handler.ExtractEventTimestamp(jsonData)

		if err != nil {
			logger.Logger.Error("Failed to extract event timestamp",
				zap.Error(err),
				zap.String("event_type", eventTypeStr),
				zap.String("delivery_id", deliveryID))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to extract event timestamp"})
			return
		}

		orderingKey, err := handler.ExtractOrderingKey(jsonData)
		if err != nil {
			logger.Logger.Error("Failed to extract ordering key",
				zap.Error(err),
				zap.String("event_type", eventTypeStr),
				zap.String("delivery_id", deliveryID))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to extract ordering key"})
			return
		}

		statusPriority, err := handler.GetStatusPriority(jsonData)
		if err != nil {
			logger.Logger.Error("Failed to extract status priority",
				zap.Error(err),
				zap.String("event_type", eventTypeStr),
				zap.String("delivery_id", deliveryID))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to extract status priority"})
			return
		}

		orderedEvent := &models.OrderedEvent{
			Sequence: models.EventSequence{
				EventID:    deliveryID,
				Timestamp:  extractedTime,
				DeliveryID: deliveryID,
				ReceivedAt: time.Now(),
			},
			EventType:      eventTypeStr,
			RawPayload:     jsonData,
			OrderingKey:    orderingKey,
			StatusPriority: statusPriority,
		}

		if err := h.orderingService.AddEvent(orderedEvent); err != nil {
			logger.Logger.Error("Failed to add event to ordering service", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process event"})
			return
		}

		logger.Logger.Debug("Event queued for ordered processing",
			zap.String("event_type", orderedEvent.EventType),
			zap.String("delivery_id", orderedEvent.Sequence.DeliveryID),
			zap.String("ordering_key", orderedEvent.OrderingKey),
			zap.Int("status_priority", orderedEvent.StatusPriority),
		)

		c.JSON(http.StatusAccepted, gin.H{"status": "queued", "message": "Event queued for processing"})
	}
}

func (h *WebhookHandler) processOrderedEvent(event *models.OrderedEvent) error {

	if err := h.db.StoreWebhookEvent(event); err != nil {
		logger.Logger.Error("Failed to store webhook event", zap.Error(err))
		//log and continue
	}

	handler, exists := h.handlers[event.EventType]

	if !exists {
		logger.Logger.Warn("No handler registered for event type", zap.String("event_type", event.EventType))
		return fmt.Errorf("event type not supported: %s", event.EventType)
	}

	jsonData := event.RawPayload

	err := handler.HandleEvent(jsonData, &event.Sequence)
	if err != nil {
		logger.Logger.Error("Failed to handle event", zap.Error(err),
			zap.String("event_type", event.EventType),
			zap.String("delivery_id", event.Sequence.DeliveryID))
		h.db.MarkEventFailed(event.Sequence.DeliveryID)
		return fmt.Errorf("failed to handle event: %w", err)
	}

	return h.db.MarkEventProcessed(event.Sequence.DeliveryID)
}

func (h *WebhookHandler) Shutdown() {
	if h.orderingService != nil {
		h.orderingService.Stop()
		logger.Logger.Info("WebhookHandler ordering service stopped")
	}
}
