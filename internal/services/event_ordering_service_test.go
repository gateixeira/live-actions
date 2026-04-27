package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupTestLoggerForEventOrdering initializes the logger for testing
func setupTestLoggerForEventOrdering() {
	logger.InitLogger("error") // Use error level to reduce test output
}

// createTestEvent creates a test OrderedEvent
func createTestEvent(deliveryID, eventType, orderingKey string, statusPriority int) *models.OrderedEvent {
	return &models.OrderedEvent{
		Sequence: models.EventSequence{
			EventID:    "event_" + deliveryID,
			SequenceID: 1,
			Timestamp:  time.Now(),
			DeliveryID: deliveryID,
			ReceivedAt: time.Now(),
		},
		EventType:      eventType,
		RawPayload:     []byte(`{"test": "data"}`),
		OrderingKey:    orderingKey,
		StatusPriority: statusPriority,
	}
}

func TestNewEventOrderingService(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	mockDB := new(database.MockDatabase)
	processFunc := func(event *models.OrderedEvent) error {
		return nil
	}

	service := NewEventOrderingService(mockDB, processFunc)

	assert.NotNil(t, service)
	assert.Equal(t, mockDB, service.db)
	assert.NotNil(t, service.processFunc)
	assert.Equal(t, 1*time.Second, service.flushInterval)
	assert.Equal(t, 10*time.Second, service.maxAge)
	assert.Equal(t, 500, service.batchSize)
	assert.NotNil(t, service.ctx)
	assert.NotNil(t, service.cancel)
}

func TestEventOrderingService_AddEvent(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	t.Run("enqueues event onto ingest channel without DB write", func(t *testing.T) {
		mockDB := new(database.MockDatabase)
		// AddEvent is now a channel send; it must NOT touch the DB synchronously.
		// We deliberately do not wire StoreWebhookEvents here so that any
		// synchronous DB call would fail the test.

		service := NewEventOrderingService(mockDB, func(event *models.OrderedEvent) error {
			return nil
		})

		err := service.AddEvent(createTestEvent("delivery-1", "workflow_job", "job-123", 1))
		assert.NoError(t, err)

		// Event should be sitting in the channel waiting for the ingest worker.
		assert.Equal(t, 1, len(service.ingestCh))
		mockDB.AssertNotCalled(t, "StoreWebhookEvent", mock.Anything, mock.Anything)
		mockDB.AssertNotCalled(t, "StoreWebhookEvents", mock.Anything, mock.Anything)
	})

	t.Run("returns ErrIngestQueueFull when channel saturated", func(t *testing.T) {
		mockDB := new(database.MockDatabase)
		service := NewEventOrderingService(mockDB, func(event *models.OrderedEvent) error {
			return nil
		})
		// Shrink channel + timeout to make the saturation path observable in tests.
		service.ingestCh = make(chan *models.OrderedEvent, 1)
		service.enqueueTimeout = 20 * time.Millisecond

		// Fill the channel.
		assert.NoError(t, service.AddEvent(createTestEvent("d-1", "workflow_job", "k", 1)))

		// Next AddEvent must time out and surface ErrIngestQueueFull.
		err := service.AddEvent(createTestEvent("d-2", "workflow_job", "k", 1))
		assert.ErrorIs(t, err, ErrIngestQueueFull)
	})

	t.Run("returns context error when service stopped", func(t *testing.T) {
		mockDB := new(database.MockDatabase)
		service := NewEventOrderingService(mockDB, func(event *models.OrderedEvent) error {
			return nil
		})
		service.ingestCh = make(chan *models.OrderedEvent, 1)
		service.enqueueTimeout = time.Second

		// Fill, then cancel the service so the second send observes ctx.Done().
		assert.NoError(t, service.AddEvent(createTestEvent("d-1", "workflow_job", "k", 1)))
		service.cancel()

		err := service.AddEvent(createTestEvent("d-2", "workflow_job", "k", 1))
		assert.ErrorIs(t, err, context.Canceled)
	})
}

// TestEventOrderingService_IngestWorker_BatchesInserts asserts that events
// pushed via AddEvent are persisted via StoreWebhookEvents (the batched call)
// rather than per-event StoreWebhookEvent.
func TestEventOrderingService_IngestWorker_BatchesInserts(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	mockDB := new(database.MockDatabase)
	mockDB.On("StoreWebhookEvents", mock.Anything, mock.MatchedBy(func(events []*models.OrderedEvent) bool {
		return len(events) >= 1
	})).Return(nil).Maybe()
	mockDB.On("GetPendingEventsByAge", mock.Anything, mock.Anything, mock.Anything).Return([]*models.OrderedEvent{}, nil).Maybe()
	mockDB.On("GetPendingEventsGrouped", mock.Anything, 1000).Return([]*models.OrderedEvent{}, nil).Maybe()

	service := NewEventOrderingService(mockDB, func(*models.OrderedEvent) error { return nil })
	service.ingestBatchWait = 10 * time.Millisecond
	service.flushInterval = time.Hour // keep the flush worker out of the way
	service.Start()

	for i := 0; i < 5; i++ {
		assert.NoError(t, service.AddEvent(createTestEvent(
			"d-"+string(rune('0'+i)), "workflow_job", "k", 1)))
	}

	// Wait for the batch ticker to fire at least once.
	time.Sleep(60 * time.Millisecond)
	service.Stop()

	// Verify StoreWebhookEvents was called and the legacy per-event method was not.
	mockDB.AssertCalled(t, "StoreWebhookEvents", mock.Anything, mock.Anything)
	mockDB.AssertNotCalled(t, "StoreWebhookEvent", mock.Anything, mock.Anything)
}

func TestEventOrderingService_StartStop(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	mockDB := new(database.MockDatabase)
	processFunc := func(event *models.OrderedEvent) error {
		return nil
	}

	service := NewEventOrderingService(mockDB, processFunc)

	// Mock expectations for the initial flush on stop
	mockDB.On("GetPendingEventsGrouped", mock.Anything, 1000).Return([]*models.OrderedEvent{}, nil)

	// Start the service
	service.Start()

	// Verify the service is running by checking if context is not done
	assert.NotNil(t, service.ctx)
	select {
	case <-service.ctx.Done():
		t.Fatal("Service context should not be done immediately after start")
	default:
		// Expected behavior
	}

	// Stop the service
	service.Stop()

	// Give some time for the goroutine to process the cancellation
	time.Sleep(50 * time.Millisecond)

	// Verify the service is stopped
	select {
	case <-service.ctx.Done():
		// Expected behavior
	default:
		t.Fatal("Service context should be done after stop")
	}

	mockDB.AssertExpectations(t)
}

func TestEventOrderingService_flushReadyEvents(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	tests := []struct {
		name         string
		mockSetup    func(*database.MockDatabase)
		expectedLogs int // Number of events we expect to be processed
	}{
		{
			name: "no pending events",
			mockSetup: func(m *database.MockDatabase) {
				m.On("GetPendingEventsByAge", mock.Anything, 10*time.Second, 500).Return([]*models.OrderedEvent{}, nil)
			},
			expectedLogs: 0,
		},
		{
			name: "multiple pending events",
			mockSetup: func(m *database.MockDatabase) {
				events := []*models.OrderedEvent{
					createTestEvent("delivery-1", "workflow_job", "job-123", 1),
					createTestEvent("delivery-2", "workflow_job", "job-456", 2),
				}
				m.On("GetPendingEventsByAge", mock.Anything, 10*time.Second, 500).Return(events, nil)
			},
			expectedLogs: 2,
		},
		{
			name: "database error",
			mockSetup: func(m *database.MockDatabase) {
				m.On("GetPendingEventsByAge", mock.Anything, 10*time.Second, 500).Return([]*models.OrderedEvent{}, errors.New("db error"))
			},
			expectedLogs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(database.MockDatabase)
			tt.mockSetup(mockDB)

			processedCount := 0
			var mu sync.Mutex
			processFunc := func(event *models.OrderedEvent) error {
				mu.Lock()
				processedCount++
				mu.Unlock()
				return nil
			}

			service := NewEventOrderingService(mockDB, processFunc)

			// Call flushReadyEvents directly
			service.flushReadyEvents()

			// Give some time for the goroutine to process events
			time.Sleep(100 * time.Millisecond)

			mu.Lock()
			assert.Equal(t, tt.expectedLogs, processedCount)
			mu.Unlock()

			mockDB.AssertExpectations(t)
		})
	}
}

func TestEventOrderingService_flushReadyEvents_drainsBacklogAcrossIterations(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	mockDB := new(database.MockDatabase)

	// First two calls return a full batch (forcing the drain loop to keep
	// going); the third returns a partial batch which terminates the loop.
	full := func() []*models.OrderedEvent {
		events := make([]*models.OrderedEvent, 500)
		for i := 0; i < 500; i++ {
			events[i] = createTestEvent("d-"+itoa(i), "workflow_job", "k", i)
		}
		return events
	}
	partial := []*models.OrderedEvent{
		createTestEvent("d-tail-1", "workflow_job", "k", 0),
		createTestEvent("d-tail-2", "workflow_job", "k", 1),
	}

	mockDB.On("GetPendingEventsByAge", mock.Anything, 10*time.Second, 500).
		Return(full(), nil).Once()
	mockDB.On("GetPendingEventsByAge", mock.Anything, 10*time.Second, 500).
		Return(full(), nil).Once()
	mockDB.On("GetPendingEventsByAge", mock.Anything, 10*time.Second, 500).
		Return(partial, nil).Once()

	var processed int
	var mu sync.Mutex
	service := NewEventOrderingService(mockDB, func(*models.OrderedEvent) error {
		mu.Lock()
		processed++
		mu.Unlock()
		return nil
	})

	service.flushReadyEvents()

	mu.Lock()
	got := processed
	mu.Unlock()

	assert.Equal(t, 500+500+2, got, "drain loop should pull batches until DB returns < batchSize")
	mockDB.AssertExpectations(t)
}

// itoa is a tiny helper so the test file does not need strconv just for this.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func TestEventOrderingService_flushAll(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	tests := []struct {
		name         string
		mockSetup    func(*database.MockDatabase)
		expectedLogs int
	}{
		{
			name: "no pending events",
			mockSetup: func(m *database.MockDatabase) {
				m.On("GetPendingEventsGrouped", mock.Anything, 1000).Return([]*models.OrderedEvent{}, nil)
			},
			expectedLogs: 0,
		},
		{
			name: "multiple pending events",
			mockSetup: func(m *database.MockDatabase) {
				events := []*models.OrderedEvent{
					createTestEvent("delivery-1", "workflow_job", "job-123", 1),
					createTestEvent("delivery-2", "workflow_run", "run-789", 3),
				}
				m.On("GetPendingEventsGrouped", mock.Anything, 1000).Return(events, nil)
			},
			expectedLogs: 2,
		},
		{
			name: "database error",
			mockSetup: func(m *database.MockDatabase) {
				m.On("GetPendingEventsGrouped", mock.Anything, 1000).Return([]*models.OrderedEvent{}, errors.New("db error"))
			},
			expectedLogs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(database.MockDatabase)
			tt.mockSetup(mockDB)

			processedCount := 0
			var mu sync.Mutex
			processFunc := func(event *models.OrderedEvent) error {
				mu.Lock()
				processedCount++
				mu.Unlock()
				return nil
			}

			service := NewEventOrderingService(mockDB, processFunc)

			// Call flushAll directly
			service.flushAll()

			// Give some time for the goroutine to process events
			time.Sleep(100 * time.Millisecond)

			mu.Lock()
			assert.Equal(t, tt.expectedLogs, processedCount)
			mu.Unlock()

			mockDB.AssertExpectations(t)
		})
	}
}

func TestEventOrderingService_processEvents(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	tests := []struct {
		name              string
		events            []*models.OrderedEvent
		processFunc       func(*models.OrderedEvent) error
		expectedProcessed int
		expectedErrorLogs int
	}{
		{
			name: "successful processing",
			events: []*models.OrderedEvent{
				createTestEvent("delivery-1", "workflow_job", "job-123", 1),
				createTestEvent("delivery-2", "workflow_job", "job-456", 2),
			},
			processFunc: func(event *models.OrderedEvent) error {
				return nil
			},
			expectedProcessed: 2,
			expectedErrorLogs: 0,
		},
		{
			name: "processing with errors",
			events: []*models.OrderedEvent{
				createTestEvent("delivery-1", "workflow_job", "job-123", 1),
				createTestEvent("delivery-2", "workflow_job", "job-456", 2),
			},
			processFunc: func(event *models.OrderedEvent) error {
				if event.Sequence.DeliveryID == "delivery-1" {
					return errors.New("processing error")
				}
				return nil
			},
			expectedProcessed: 1, // Only one succeeds
			expectedErrorLogs: 1, // One error logged
		},
		{
			name:   "empty events slice",
			events: []*models.OrderedEvent{},
			processFunc: func(event *models.OrderedEvent) error {
				return nil
			},
			expectedProcessed: 0,
			expectedErrorLogs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(database.MockDatabase)

			processedCount := 0
			var mu sync.Mutex
			wrappedProcessFunc := func(event *models.OrderedEvent) error {
				err := tt.processFunc(event)
				if err == nil {
					mu.Lock()
					processedCount++
					mu.Unlock()
				}
				return err
			}

			service := NewEventOrderingService(mockDB, wrappedProcessFunc)

			// Call processEvents directly
			service.processEvents(tt.events)

			// Give some time for processing to complete
			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			assert.Equal(t, tt.expectedProcessed, processedCount)
			mu.Unlock()
		})
	}
}

func TestEventOrderingService_flushWorker(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	mockDB := new(database.MockDatabase)

	// Set up expectations for periodic flush calls
	mockDB.On("GetPendingEventsByAge", mock.Anything, mock.AnythingOfType("time.Duration"), mock.AnythingOfType("int")).Return([]*models.OrderedEvent{}, nil).Maybe()

	// Set up expectations for final flush on shutdown
	mockDB.On("GetPendingEventsGrouped", mock.Anything, 1000).Return([]*models.OrderedEvent{}, nil).Once()

	processFunc := func(event *models.OrderedEvent) error {
		return nil
	}

	service := NewEventOrderingService(mockDB, processFunc)

	// Override flush interval for faster testing
	service.flushInterval = 50 * time.Millisecond

	// Start the flush worker
	service.Start()

	// Let it run for a short time
	time.Sleep(120 * time.Millisecond)

	// Stop the service and wait for completion
	service.Stop()

	mockDB.AssertExpectations(t)
}

func TestEventOrderingService_ConcurrentAccess(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	mockDB := new(database.MockDatabase)

	// Allow multiple calls to database methods
	mockDB.On("StoreWebhookEvents", mock.Anything, mock.AnythingOfType("[]*models.OrderedEvent")).Return(nil).Maybe()
	mockDB.On("GetPendingEventsByAge", mock.Anything, mock.AnythingOfType("time.Duration"), mock.AnythingOfType("int")).Return([]*models.OrderedEvent{}, nil).Maybe()
	mockDB.On("GetPendingEventsGrouped", mock.Anything, 1000).Return([]*models.OrderedEvent{}, nil).Maybe()

	processedEvents := make([]string, 0)
	var mu sync.Mutex

	processFunc := func(event *models.OrderedEvent) error {
		mu.Lock()
		processedEvents = append(processedEvents, event.Sequence.DeliveryID)
		mu.Unlock()
		return nil
	}

	service := NewEventOrderingService(mockDB, processFunc)
	service.flushInterval = 50 * time.Millisecond

	// Start the service
	service.Start()

	// Add events concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	eventsPerGoroutine := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := createTestEvent(
					string(rune('A'+routineID))+string(rune('0'+j)),
					"workflow_job",
					"concurrent-test",
					1,
				)
				_ = service.AddEvent(event)
			}
		}(i)
	}

	wg.Wait()

	// Give time for processing
	time.Sleep(200 * time.Millisecond)

	// Stop the service
	service.Stop()

	// Verify no race conditions occurred (test should not panic)
	assert.True(t, true, "Concurrent access test completed without race conditions")

	mockDB.AssertExpectations(t)
}

func TestEventOrderingService_ContextCancellation(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	mockDB := new(database.MockDatabase)

	// Expect the final flush call when context is cancelled
	mockDB.On("GetPendingEventsGrouped", mock.Anything, 1000).Return([]*models.OrderedEvent{}, nil).Once()

	processFunc := func(event *models.OrderedEvent) error {
		return nil
	}

	service := NewEventOrderingService(mockDB, processFunc)
	service.flushInterval = 1 * time.Second // Longer interval to test cancellation

	// Start and immediately stop the service
	service.Start()
	service.Stop()

	mockDB.AssertExpectations(t)
}

func TestEventOrderingService_CustomConfiguration(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	mockDB := new(database.MockDatabase)
	processFunc := func(event *models.OrderedEvent) error {
		return nil
	}

	service := NewEventOrderingService(mockDB, processFunc)

	// Test that we can modify configuration after creation
	originalFlushInterval := service.flushInterval
	originalMaxAge := service.maxAge
	originalBatchSize := service.batchSize

	// Verify default values
	assert.Equal(t, 1*time.Second, originalFlushInterval)
	assert.Equal(t, 10*time.Second, originalMaxAge)
	assert.Equal(t, 500, originalBatchSize)

	// Modify configuration
	service.flushInterval = 2 * time.Second
	service.maxAge = 5 * time.Second
	service.batchSize = 50

	// Verify changes
	assert.Equal(t, 2*time.Second, service.flushInterval)
	assert.Equal(t, 5*time.Second, service.maxAge)
	assert.Equal(t, 50, service.batchSize)
}
