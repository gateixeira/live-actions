package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupWebhookTest() (*gin.Engine, *config.Config) {
	logger.InitLogger("error")
	gin.SetMode(gin.TestMode)
	router := gin.New()

	testConfig := &config.Config{
		Vars: config.Vars{
			WebhookSecret: "test-secret",
		},
	}

	return router, testConfig
}

func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestValidateGitHubWebhook_OversizedBody(t *testing.T) {
	router, testConfig := setupWebhookTest()
	router.POST("/webhook", ValidateGitHubWebhook(testConfig), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Create a body larger than 10 MB
	largeBody := make([]byte, 11*1024*1024)
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	signature := signPayload(testConfig.Vars.WebhookSecret, largeBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", bytes.NewReader(largeBody))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "workflow_job")
	req.Header.Set("X-GitHub-Delivery", "test-delivery-id")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "Request body too large")
}

func TestValidateGitHubWebhook_ValidBody(t *testing.T) {
	router, testConfig := setupWebhookTest()
	router.POST("/webhook", ValidateGitHubWebhook(testConfig), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := []byte(`{"action":"queued","workflow_job":{"id":1}}`)
	signature := signPayload(testConfig.Vars.WebhookSecret, body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "workflow_job")
	req.Header.Set("X-GitHub-Delivery", "test-delivery-id")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidateGitHubWebhook_MissingSecret(t *testing.T) {
	router, _ := setupWebhookTest()
	emptyConfig := &config.Config{
		Vars: config.Vars{WebhookSecret: ""},
	}
	router.POST("/webhook", ValidateGitHubWebhook(emptyConfig), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-Hub-Signature-256", "sha256=abc")
	req.Header.Set("X-GitHub-Event", "workflow_job")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Webhook secret not configured")
}

func TestValidateGitHubWebhook_MissingSignature(t *testing.T) {
	router, testConfig := setupWebhookTest()
	router.POST("/webhook", ValidateGitHubWebhook(testConfig), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-GitHub-Event", "workflow_job")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Missing signature header")
}

func TestValidateGitHubWebhook_InvalidSignature(t *testing.T) {
	router, testConfig := setupWebhookTest()
	router.POST("/webhook", ValidateGitHubWebhook(testConfig), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := []byte(`{"action":"queued"}`)
	// Use wrong secret to generate invalid signature
	wrongSignature := signPayload("wrong-secret", body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", wrongSignature)
	req.Header.Set("X-GitHub-Event", "workflow_job")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid signature")
}

func TestValidateGitHubWebhook_MissingEventType(t *testing.T) {
	router, testConfig := setupWebhookTest()
	router.POST("/webhook", ValidateGitHubWebhook(testConfig), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := []byte(`{"action":"queued"}`)
	signature := signPayload(testConfig.Vars.WebhookSecret, body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", signature)
	// No X-GitHub-Event header
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing event type")
}
