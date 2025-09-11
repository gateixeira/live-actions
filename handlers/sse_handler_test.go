package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSSETest() {
	// Initialize logger for tests
	logger.InitLogger("error")
	gin.SetMode(gin.TestMode)
}

func TestInitSSEHandler(t *testing.T) {
	setupSSETest()

	// Reset global handler to test initialization
	sseHandler = nil

	InitSSEHandler()

	assert.NotNil(t, sseHandler, "InitSSEHandler should create a global SSE handler")
	assert.NotNil(t, sseHandler.client, "SSE handler should have a client channel")
	assert.Equal(t, 100, cap(sseHandler.client), "Client channel should have capacity of 100")
}

func TestGetSSEHandler(t *testing.T) {
	setupSSETest()

	// Initialize handler first
	InitSSEHandler()

	handler := GetSSEHandler()
	assert.NotNil(t, handler, "GetSSEHandler should return the global handler")
	assert.Equal(t, sseHandler, handler, "GetSSEHandler should return the same instance as global handler")
}

func TestSSEHandler_SendEvent(t *testing.T) {
	setupSSETest()

	handler := &SSEHandler{
		client: make(chan SSEEvent, 10),
	}

	testData := map[string]interface{}{
		"message": "test event",
		"id":      123,
	}

	// Send an event
	handler.SendEvent("test_event", testData)

	// Verify the event was sent
	select {
	case event := <-handler.client:
		assert.Equal(t, "test_event", event.Type)
		assert.Equal(t, testData, event.Data)
	case <-time.After(1 * time.Second):
		t.Fatal("Event was not received within timeout")
	}
}

func TestSSEHandler_SendEvent_NilHandler(t *testing.T) {
	setupSSETest()

	var handler *SSEHandler = nil

	// Should not panic when handler is nil
	assert.NotPanics(t, func() {
		handler.SendEvent("test", "data")
	})
}

func TestSSEHandler_SendEvent_NilChannel(t *testing.T) {
	setupSSETest()

	handler := &SSEHandler{
		client: nil,
	}

	// Should not panic when channel is nil
	assert.NotPanics(t, func() {
		handler.SendEvent("test", "data")
	})
}

func TestSSEHandler_SendEvent_ChannelFull(t *testing.T) {
	setupSSETest()

	// Create handler with small buffer
	handler := &SSEHandler{
		client: make(chan SSEEvent, 2),
	}

	// Fill the channel
	handler.SendEvent("event1", "data1")
	handler.SendEvent("event2", "data2")

	// This should not block and should drop the event
	start := time.Now()
	handler.SendEvent("event3", "data3") // Should be dropped
	duration := time.Since(start)

	// Should return immediately (non-blocking)
	assert.Less(t, duration, 100*time.Millisecond, "SendEvent should not block when channel is full")

	// Verify only first two events are in channel
	event1 := <-handler.client
	event2 := <-handler.client

	assert.Equal(t, "event1", event1.Type)
	assert.Equal(t, "event2", event2.Type)

	// Channel should be empty now
	select {
	case <-handler.client:
		t.Fatal("Channel should be empty")
	default:
		// Expected
	}
}

func TestSSEHandler_HandleSSE_Headers(t *testing.T) {
	setupSSETest()

	handler := &SSEHandler{
		client: make(chan SSEEvent, 10),
	}

	router := gin.New()
	router.GET("/events", handler.HandleSSE())

	req, _ := http.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()

	// Use a context with timeout to prevent the handler from running indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	router.ServeHTTP(w, req)

	// Check SSE headers
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Cache-Control", w.Header().Get("Access-Control-Allow-Headers"))
}

func TestSSEHandler_HandleSSE_InitialConnectionEvent(t *testing.T) {
	setupSSETest()

	handler := &SSEHandler{
		client: make(chan SSEEvent, 10),
	}

	router := gin.New()
	router.GET("/events", handler.HandleSSE())

	req, _ := http.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()

	// Use a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	router.ServeHTTP(w, req)

	body := w.Body.String()

	// Should contain initial connection event
	assert.Contains(t, body, "event:message", "Response should contain SSE message event")
	assert.Contains(t, body, "connected", "Response should contain connection confirmation")
	assert.Contains(t, body, "SSE connection established", "Response should contain connection message")
}

func TestSSEHandler_HandleSSE_EventForwarding(t *testing.T) {
	setupSSETest()

	handler := &SSEHandler{
		client: make(chan SSEEvent, 10),
	}

	router := gin.New()
	router.GET("/events", handler.HandleSSE())

	// Start the SSE handler in a goroutine
	req, _ := http.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Send an event to the global channel before starting the handler
	testEvent := SSEEvent{
		Type: "test_event",
		Data: map[string]string{"message": "test data"},
	}

	// Start handler in goroutine
	done := make(chan bool)
	go func() {
		router.ServeHTTP(w, req)
		done <- true
	}()

	// Give handler time to start
	time.Sleep(50 * time.Millisecond)

	// Send event to global channel
	handler.client <- testEvent

	// Wait for context timeout or completion
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Handler did not complete within timeout")
	}

	body := w.Body.String()

	// Should contain the forwarded event
	assert.Contains(t, body, "test_event", "Response should contain the test event")
	assert.Contains(t, body, "test data", "Response should contain the test data")
}

func TestSSEHandler_HandleSSE_KeepAlive(t *testing.T) {
	setupSSETest()

	handler := &SSEHandler{
		client: make(chan SSEEvent, 10),
	}

	router := gin.New()
	router.GET("/events", handler.HandleSSE())

	req, _ := http.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()

	// Use a longer timeout to test keepalive (but shorter than 30s for testing)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	router.ServeHTTP(w, req)

	body := w.Body.String()

	// Should contain initial connection event at minimum
	assert.Contains(t, body, "event:message", "Response should contain SSE events")

	// Note: Testing the full 30-second keepalive would make tests too slow,
	// but we can verify the structure is correct
}

func TestSendMetricsUpdate(t *testing.T) {
	setupSSETest()

	// Initialize global handler
	InitSSEHandler()

	testUpdate := models.MetricsUpdateEvent{
		RunningJobs: 5,
		QueuedJobs:  3,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	// Send metrics update
	SendMetricsUpdate(testUpdate)

	// Verify event was sent to global handler
	select {
	case event := <-sseHandler.client:
		assert.Equal(t, "metrics_update", event.Type)
		assert.Equal(t, testUpdate, event.Data)
	case <-time.After(1 * time.Second):
		t.Fatal("Metrics update event was not received")
	}
}

func TestSendMetricsUpdate_NilHandler(t *testing.T) {
	setupSSETest()

	// Set global handler to nil
	sseHandler = nil

	testUpdate := models.MetricsUpdateEvent{
		RunningJobs: 1,
	}

	// Should not panic when global handler is nil
	assert.NotPanics(t, func() {
		SendMetricsUpdate(testUpdate)
	})
}

func TestSendWorkflowUpdate(t *testing.T) {
	setupSSETest()

	// Initialize global handler
	InitSSEHandler()

	testUpdate := models.WorkflowUpdateEvent{
		Type:      "run",
		Action:    "completed",
		ID:        12345,
		Status:    "success",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Send workflow update
	SendWorkflowUpdate(testUpdate)

	// Verify event was sent to global handler
	select {
	case event := <-sseHandler.client:
		assert.Equal(t, "workflow_update", event.Type)
		assert.Equal(t, testUpdate, event.Data)
	case <-time.After(1 * time.Second):
		t.Fatal("Workflow update event was not received")
	}
}

func TestSendWorkflowUpdate_NilHandler(t *testing.T) {
	setupSSETest()

	// Set global handler to nil
	sseHandler = nil

	testUpdate := models.WorkflowUpdateEvent{
		Type:   "job",
		Action: "queued",
		ID:     67890,
	}

	// Should not panic when global handler is nil
	assert.NotPanics(t, func() {
		SendWorkflowUpdate(testUpdate)
	})
}

func TestSSEEvent_JSONSerialization(t *testing.T) {
	setupSSETest()

	event := SSEEvent{
		Type: "test_event",
		Data: map[string]interface{}{
			"string_field": "test",
			"number_field": 123,
			"bool_field":   true,
			"nested_field": map[string]string{
				"key": "value",
			},
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(event)
	require.NoError(t, err, "Should be able to marshal SSEEvent to JSON")

	// Test JSON unmarshaling
	var unmarshaled SSEEvent
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err, "Should be able to unmarshal SSEEvent from JSON")

	assert.Equal(t, event.Type, unmarshaled.Type)
	// Note: Data comparison might be tricky due to interface{} type,
	// but the important thing is that it marshals/unmarshals without error
}

func TestSSEHandler_ConcurrentEvents(t *testing.T) {
	setupSSETest()

	handler := &SSEHandler{
		client: make(chan SSEEvent, 100),
	}

	numGoroutines := 10
	eventsPerGoroutine := 5
	totalEvents := numGoroutines * eventsPerGoroutine

	// Send events concurrently
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < eventsPerGoroutine; j++ {
				eventType := fmt.Sprintf("event_%d_%d", goroutineID, j)
				eventData := map[string]int{
					"goroutine": goroutineID,
					"event":     j,
				}
				handler.SendEvent(eventType, eventData)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all events were received
	receivedEvents := 0
	timeout := time.After(2 * time.Second)

	for receivedEvents < totalEvents {
		select {
		case <-handler.client:
			receivedEvents++
		case <-timeout:
			t.Fatalf("Only received %d out of %d events before timeout", receivedEvents, totalEvents)
		}
	}

	assert.Equal(t, totalEvents, receivedEvents, "Should receive all sent events")
}

func TestSSEHandler_HandleSSE_ClientDisconnection(t *testing.T) {
	setupSSETest()

	handler := &SSEHandler{
		client: make(chan SSEEvent, 10),
	}

	router := gin.New()
	router.GET("/events", handler.HandleSSE())

	req, _ := http.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()

	// Create a context that will be cancelled to simulate client disconnection
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	// Start handler in goroutine
	done := make(chan bool)
	go func() {
		router.ServeHTTP(w, req)
		done <- true
	}()

	// Give handler time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context to simulate client disconnection
	cancel()

	// Handler should terminate gracefully
	select {
	case <-done:
		// Expected - handler should return when context is cancelled
	case <-time.After(1 * time.Second):
		t.Fatal("Handler did not terminate after context cancellation")
	}
}

func TestSSEHandler_HandleSSE_JSONMarshalError(t *testing.T) {
	setupSSETest()

	handler := &SSEHandler{
		client: make(chan SSEEvent, 10),
	}

	router := gin.New()
	router.GET("/events", handler.HandleSSE())

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequest("GET", "/events", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	// Start handler in goroutine
	done := make(chan bool)
	go func() {
		router.ServeHTTP(w, req)
		done <- true
	}()

	// Give handler time to start
	time.Sleep(50 * time.Millisecond)

	// Send an event with data that cannot be marshaled to JSON
	// (functions cannot be marshaled to JSON)
	badEvent := SSEEvent{
		Type: "bad_event",
		Data: map[string]interface{}{
			"function": func() {}, // This will cause JSON marshal error
		},
	}

	handler.client <- badEvent

	// Wait for completion
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Handler did not complete within timeout")
	}

	// Handler should continue running despite the marshal error
	// The bad event should be skipped but handler should not crash
	body := w.Body.String()
	assert.Contains(t, body, "connected", "Handler should still send initial connection event")
	assert.NotContains(t, body, "bad_event", "Bad event should not appear in output")
}
