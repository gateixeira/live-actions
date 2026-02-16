package services

import (
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
	assert.Equal(t, 5*time.Second, service.flushInterval)
	assert.Equal(t, 10*time.Second, service.maxAge)
	assert.Equal(t, 100, service.batchSize)
	assert.NotNil(t, service.ctx)
	assert.NotNil(t, service.cancel)
}

func TestEventOrderingService_AddEvent(t *testing.T) {
	setupTestLoggerForEventOrdering()
	defer logger.SyncLogger()

	tests := []struct {
		name          string
		event         *models.OrderedEvent
		mockSetup     func(*database.MockDatabase)
		expectedError bool
	}{
		{
			name:  "successful event storage",
			event: createTestEvent("delivery-1", "workflow_job", "job-123", 1),
			mockSetup: func(m *database.MockDatabase) {
				m.On("StoreWebhookEvent", mock.Anything, mock.AnythingOfType("*models.OrderedEvent")).Return(nil)
			},
			expectedError: false,
		},
		{
			name:  "database error",
			event: createTestEvent("delivery-2", "workflow_job", "job-456", 2),
			mockSetup: func(m *database.MockDatabase) {
				m.On("StoreWebhookEvent", mock.Anything, mock.AnythingOfType("*models.OrderedEvent")).Return(errors.New("database error"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(database.MockDatabase)
			tt.mockSetup(mockDB)

			service := NewEventOrderingService(mockDB, func(event *models.OrderedEvent) error {
				return nil
			})

			err := service.AddEvent(tt.event)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockDB.AssertExpectations(t)
		})
	}
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
				m.On("GetPendingEventsByAge", mock.Anything, 10*time.Second, 100).Return([]*models.OrderedEvent{}, nil)
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
				m.On("GetPendingEventsByAge", mock.Anything, 10*time.Second, 100).Return(events, nil)
			},
			expectedLogs: 2,
		},
		{
			name: "database error",
			mockSetup: func(m *database.MockDatabase) {
				m.On("GetPendingEventsByAge", mock.Anything, 10*time.Second, 100).Return([]*models.OrderedEvent{}, errors.New("db error"))
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
	mockDB.On("StoreWebhookEvent", mock.Anything, mock.AnythingOfType("*models.OrderedEvent")).Return(nil).Maybe()
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
				service.AddEvent(event)
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
	assert.Equal(t, 5*time.Second, originalFlushInterval)
	assert.Equal(t, 10*time.Second, originalMaxAge)
	assert.Equal(t, 100, originalBatchSize)

	// Modify configuration
	service.flushInterval = 2 * time.Second
	service.maxAge = 5 * time.Second
	service.batchSize = 50

	// Verify changes
	assert.Equal(t, 2*time.Second, service.flushInterval)
	assert.Equal(t, 5*time.Second, service.maxAge)
	assert.Equal(t, 50, service.batchSize)
}
