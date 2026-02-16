package handlers

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupWorkflowJobTest() (*database.MockDatabase, *config.Config) {
	// Initialize logger for tests
	logger.InitLogger("error")

	// Initialize SSE handler to prevent panics
	InitSSEHandler()

	testConfig := &config.Config{
		Vars: config.Vars{},
	}

	return &database.MockDatabase{}, testConfig
}

func TestNewWorkflowJobHandler(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	assert.NotNil(t, handler, "NewWorkflowJobHandler should return a non-nil handler")
	assert.Equal(t, mockDB, handler.db, "Handler should store the database interface")
}

func TestWorkflowJobHandler_GetEventType(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	eventType := handler.GetEventType()
	assert.Equal(t, "workflow_job", eventType, "GetEventType should return 'workflow_job'")
}

func TestWorkflowJobHandler_HandleEvent_Success(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	// Create test data
	now := time.Now()
	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	workflowJobEvent := models.WorkflowJobEvent{
		Action: "in_progress",
		WorkflowJob: models.WorkflowJob{
			ID:          12345,
			Name:        "Test Job",
			Status:      models.JobStatusQueued, // This will be overridden by action
			Labels:      []string{"ubuntu-latest", "self-hosted"},
			Conclusion:  "",
			CreatedAt:   now,
			StartedAt:   now,
			CompletedAt: time.Time{},
			RunID:       67890,
		},
	}

	eventData, err := json.Marshal(workflowJobEvent)
	assert.NoError(t, err, "Should be able to marshal test data")

	// Set up mock expectations
	mockDB.On("GetWorkflowJobByID", mock.Anything, int64(12345)).Return(models.WorkflowJob{
		Status: models.JobStatusQueued, // Previous status
	}, nil)

	mockDB.On("AddOrUpdateJob", mock.Anything, mock.MatchedBy(func(job models.WorkflowJob) bool {
		return job.ID == 12345 &&
			job.Name == "Test Job" &&
			job.Status == models.JobStatus("in_progress") &&
			job.RunID == 67890
	}), mock.AnythingOfType("time.Time")).Return(true, nil)

	// Set up mock expectations for metrics update
	mockDB.On("GetCurrentJobCounts", mock.Anything).Return(1, 2, nil)

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Verify results
	assert.NoError(t, err, "HandleEvent should not return an error")
	mockDB.AssertExpectations(t)
}

func TestWorkflowJobHandler_HandleEvent_InvalidJSON(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	sequence := &models.EventSequence{
		EventID:    "event123",
		DeliveryID: "delivery123",
	}

	invalidJSON := []byte(`{"invalid": json`)

	err := handler.HandleEvent(invalidJSON, sequence)

	assert.Error(t, err, "HandleEvent should return an error for invalid JSON")
	assert.Contains(t, err.Error(), "invalid JSON payload", "Error should mention invalid JSON")
	mockDB.AssertExpectations(t) // No database calls should have been made
}

func TestWorkflowJobHandler_HandleEvent_DatabaseGetJobError(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	now := time.Now()
	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	workflowJobEvent := models.WorkflowJobEvent{
		Action: "queued",
		WorkflowJob: models.WorkflowJob{
			ID:        12345,
			Name:      "Test Job",
			Status:    models.JobStatusInProgress,
			Labels:    []string{"ubuntu-latest"},
			CreatedAt: now,
			RunID:     67890,
		},
	}

	eventData, err := json.Marshal(workflowJobEvent)
	assert.NoError(t, err, "Should be able to marshal test data")

	// Set up mock expectations - database error for GetWorkflowJobByID
	mockDB.On("GetWorkflowJobByID", mock.Anything, int64(12345)).Return(models.WorkflowJob{}, errors.New("database connection failed"))

	// Should still proceed with AddOrUpdateJob
	mockDB.On("AddOrUpdateJob", mock.Anything, mock.MatchedBy(func(job models.WorkflowJob) bool {
		return job.ID == 12345 &&
			job.Status == models.JobStatus("queued")
	}), mock.AnythingOfType("time.Time")).Return(true, nil)

	// Set up mock expectations for metrics update
	mockDB.On("GetCurrentJobCounts", mock.Anything).Return(0, 1, nil)

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Should not fail even if GetWorkflowJobByID fails
	assert.NoError(t, err, "HandleEvent should continue processing even if GetWorkflowJobByID fails")
	mockDB.AssertExpectations(t)
}

func TestWorkflowJobHandler_HandleEvent_DatabaseAddJobError(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	now := time.Now()
	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	workflowJobEvent := models.WorkflowJobEvent{
		Action: "completed",
		WorkflowJob: models.WorkflowJob{
			ID:          12345,
			Name:        "Test Job",
			Status:      models.JobStatusInProgress,
			Labels:      []string{"ubuntu-latest"},
			Conclusion:  "success",
			CreatedAt:   now,
			StartedAt:   now,
			CompletedAt: now,
			RunID:       67890,
		},
	}

	eventData, err := json.Marshal(workflowJobEvent)
	assert.NoError(t, err, "Should be able to marshal test data")

	// Set up mock expectations
	mockDB.On("GetWorkflowJobByID", mock.Anything, int64(12345)).Return(models.WorkflowJob{
		Status: models.JobStatusInProgress,
	}, nil)

	mockDB.On("AddOrUpdateJob", mock.Anything, mock.AnythingOfType("models.WorkflowJob"), mock.AnythingOfType("time.Time")).Return(false, errors.New("database save failed"))

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Should not fail even if AddOrUpdateJob fails
	assert.NoError(t, err, "HandleEvent should continue processing even if AddOrUpdateJob fails")
	mockDB.AssertExpectations(t)
}

func TestWorkflowJobHandler_HandleEvent_DifferentActions(t *testing.T) {
	testCases := []struct {
		name           string
		action         string
		expectedStatus models.JobStatus
		labels         []string
	}{
		{
			name:           "queued action",
			action:         "queued",
			expectedStatus: models.JobStatus("queued"),
			labels:         []string{"ubuntu-latest"},
		},
		{
			name:           "in_progress action",
			action:         "in_progress",
			expectedStatus: models.JobStatus("in_progress"),
			labels:         []string{"self-hosted", "linux"},
		},
		{
			name:           "completed action",
			action:         "completed",
			expectedStatus: models.JobStatus("completed"),
			labels:         []string{"windows-latest"},
		},
		{
			name:           "cancelled action",
			action:         "cancelled",
			expectedStatus: models.JobStatus("cancelled"),
			labels:         []string{"self-hosted-large"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB, testConfig := setupWorkflowJobTest()
			handler := NewWorkflowJobHandler(testConfig, mockDB)

			now := time.Now()
			sequence := &models.EventSequence{
				EventID:    "event123",
				SequenceID: 1,
				Timestamp:  now,
				DeliveryID: "delivery123",
				ReceivedAt: now,
			}

			workflowJobEvent := models.WorkflowJobEvent{
				Action: tc.action,
				WorkflowJob: models.WorkflowJob{
					ID:        12345,
					Name:      "Test Job",
					Status:    models.JobStatusQueued, // This will be overridden
					Labels:    tc.labels,
					CreatedAt: now,
					RunID:     67890,
				},
			}

			eventData, err := json.Marshal(workflowJobEvent)
			assert.NoError(t, err, "Should be able to marshal test data")

			// Set up mock expectations
			mockDB.On("GetWorkflowJobByID", mock.Anything, int64(12345)).Return(models.WorkflowJob{
				Status: models.JobStatusQueued,
			}, nil)

			mockDB.On("AddOrUpdateJob", mock.Anything, mock.MatchedBy(func(job models.WorkflowJob) bool {
				return job.ID == 12345 &&
					job.Status == tc.expectedStatus
			}), mock.AnythingOfType("time.Time")).Return(true, nil)

			// Set up mock expectations for metrics update
			mockDB.On("GetCurrentJobCounts", mock.Anything).Return(1, 0, nil)

			// Execute the handler
			err = handler.HandleEvent(eventData, sequence)

			// Verify results
			assert.NoError(t, err, "HandleEvent should not return an error")
			mockDB.AssertExpectations(t)
		})
	}
}

func TestWorkflowJobHandler_HandleEvent_StatusTransitions(t *testing.T) {
	testCases := []struct {
		name           string
		previousStatus models.JobStatus
		currentAction  string
		expectedStatus models.JobStatus
	}{
		{
			name:           "queued to in_progress",
			previousStatus: models.JobStatusQueued,
			currentAction:  "in_progress",
			expectedStatus: models.JobStatus("in_progress"),
		},
		{
			name:           "in_progress to completed",
			previousStatus: models.JobStatusInProgress,
			currentAction:  "completed",
			expectedStatus: models.JobStatus("completed"),
		},
		{
			name:           "in_progress to cancelled",
			previousStatus: models.JobStatusInProgress,
			currentAction:  "cancelled",
			expectedStatus: models.JobStatus("cancelled"),
		},
		{
			name:           "completed to queued (retry)",
			previousStatus: models.JobStatusCompleted,
			currentAction:  "queued",
			expectedStatus: models.JobStatus("queued"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB, testConfig := setupWorkflowJobTest()
			handler := NewWorkflowJobHandler(testConfig, mockDB)

			now := time.Now()
			sequence := &models.EventSequence{
				EventID:    "event123",
				SequenceID: 1,
				Timestamp:  now,
				DeliveryID: "delivery123",
				ReceivedAt: now,
			}

			workflowJobEvent := models.WorkflowJobEvent{
				Action: tc.currentAction,
				WorkflowJob: models.WorkflowJob{
					ID:        12345,
					Name:      "Test Job",
					Status:    models.JobStatusQueued, // Will be overridden
					Labels:    []string{"ubuntu-latest"},
					CreatedAt: now,
					StartedAt: now,
					RunID:     67890,
				},
			}

			eventData, err := json.Marshal(workflowJobEvent)
			assert.NoError(t, err, "Should be able to marshal test data")

			// Set up mock expectations
			mockDB.On("GetWorkflowJobByID", mock.Anything, int64(12345)).Return(models.WorkflowJob{
				Status: tc.previousStatus,
			}, nil)

			mockDB.On("AddOrUpdateJob", mock.Anything, mock.MatchedBy(func(job models.WorkflowJob) bool {
				return job.ID == 12345 &&
					job.Status == tc.expectedStatus
			}), mock.AnythingOfType("time.Time")).Return(true, nil)

			// Set up mock expectations for metrics update
			mockDB.On("GetCurrentJobCounts", mock.Anything).Return(1, 0, nil)

			// Execute the handler
			err = handler.HandleEvent(eventData, sequence)

			// Verify results
			assert.NoError(t, err, "HandleEvent should handle status transitions correctly")
			mockDB.AssertExpectations(t)
		})
	}
}

func TestWorkflowJobHandler_HandleEvent_WithStartedAtTime(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	now := time.Now()
	createdAt := now.Add(-5 * time.Minute)
	startedAt := now.Add(-2 * time.Minute)

	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	workflowJobEvent := models.WorkflowJobEvent{
		Action: "in_progress",
		WorkflowJob: models.WorkflowJob{
			ID:        12345,
			Name:      "Test Job",
			Status:    models.JobStatusQueued,
			Labels:    []string{"ubuntu-latest"},
			CreatedAt: createdAt,
			StartedAt: startedAt, // This should be used for queue time calculation
			RunID:     67890,
		},
	}

	eventData, err := json.Marshal(workflowJobEvent)
	assert.NoError(t, err, "Should be able to marshal test data")

	// Set up mock expectations
	mockDB.On("GetWorkflowJobByID", mock.Anything, int64(12345)).Return(models.WorkflowJob{
		Status: models.JobStatusQueued,
	}, nil)

	// Capture the job to verify StartedAt is preserved
	var capturedJob models.WorkflowJob
	mockDB.On("AddOrUpdateJob", mock.Anything, mock.MatchedBy(func(job models.WorkflowJob) bool {
		capturedJob = job
		return job.ID == 12345 &&
			job.Status == models.JobStatus("in_progress")
	}), mock.AnythingOfType("time.Time")).Return(true, nil)

	// Set up mock expectations for metrics update
	mockDB.On("GetCurrentJobCounts", mock.Anything).Return(1, 0, nil)

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Verify results
	assert.NoError(t, err, "HandleEvent should not return an error")
	assert.Equal(t, startedAt.Unix(), capturedJob.StartedAt.Unix(), "StartedAt should be preserved")
	assert.Equal(t, createdAt.Unix(), capturedJob.CreatedAt.Unix(), "CreatedAt should be preserved")
	mockDB.AssertExpectations(t)
}

func TestWorkflowJobHandler_HandleEvent_GetCurrentJobCountsError(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	now := time.Now()
	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	workflowJobEvent := models.WorkflowJobEvent{
		Action: "completed",
		WorkflowJob: models.WorkflowJob{
			ID:          12345,
			Name:        "Test Job",
			Status:      models.JobStatusInProgress,
			Labels:      []string{"ubuntu-latest"},
			Conclusion:  "success",
			CreatedAt:   now,
			CompletedAt: now,
			RunID:       67890,
		},
	}

	eventData, err := json.Marshal(workflowJobEvent)
	assert.NoError(t, err, "Should be able to marshal test data")

	// Set up mock expectations
	mockDB.On("GetWorkflowJobByID", mock.Anything, int64(12345)).Return(models.WorkflowJob{
		Status: models.JobStatusInProgress,
	}, nil)

	mockDB.On("AddOrUpdateJob", mock.Anything, mock.AnythingOfType("models.WorkflowJob"), mock.AnythingOfType("time.Time")).Return(true, nil)

	// Set up mock expectations for metrics update
	mockDB.On("GetCurrentJobCounts", mock.Anything).Return(0, 0, errors.New("database error"))

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Should not return error when GetCurrentJobCounts fails
	assert.NoError(t, err, "HandleEvent should not return an error when metrics update fails")
	mockDB.AssertExpectations(t)
}

func TestWorkflowJobHandler_HandleEvent_EmptyEventData(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	sequence := &models.EventSequence{
		EventID:    "event123",
		DeliveryID: "delivery123",
	}

	err := handler.HandleEvent([]byte(""), sequence)

	assert.Error(t, err, "HandleEvent should return an error for empty data")
	assert.Contains(t, err.Error(), "invalid JSON payload", "Error should mention invalid JSON")
	mockDB.AssertExpectations(t) // No database calls should have been made
}

func TestWorkflowJobHandler_HandleEvent_MinimalRequiredFields(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	now := time.Now()
	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	// Create minimal workflow job event with only required fields
	workflowJobEvent := models.WorkflowJobEvent{
		Action: "queued",
		WorkflowJob: models.WorkflowJob{
			ID:        1,
			Name:      "Minimal Job",
			Status:    models.JobStatusCompleted, // Will be overridden
			Labels:    []string{"ubuntu-latest"},
			CreatedAt: now,
			RunID:     1,
		},
	}

	eventData, err := json.Marshal(workflowJobEvent)
	assert.NoError(t, err, "Should be able to marshal minimal test data")

	// Set up mock expectations
	mockDB.On("GetWorkflowJobByID", mock.Anything, int64(1)).Return(models.WorkflowJob{}, nil)

	mockDB.On("AddOrUpdateJob", mock.Anything, mock.MatchedBy(func(job models.WorkflowJob) bool {
		return job.ID == 1 &&
			job.Status == models.JobStatus("queued")
	}), mock.AnythingOfType("time.Time")).Return(true, nil)

	// Set up mock expectations for metrics update
	mockDB.On("GetCurrentJobCounts", mock.Anything).Return(0, 1, nil)

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Verify results
	assert.NoError(t, err, "HandleEvent should work with minimal required fields")
	mockDB.AssertExpectations(t)
}

func TestWorkflowJobHandler_ExtractEventTimestamp(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	testCases := []struct {
		name           string
		eventData      []byte
		expectedTime   time.Time
		expectError    bool
		errorSubstring string
	}{
		{
			name: "valid event with timestamp",
			eventData: func() []byte {
				now := time.Date(2023, 10, 15, 14, 30, 45, 0, time.UTC)
				event := models.WorkflowJobEvent{
					Action: "queued",
					WorkflowJob: models.WorkflowJob{
						ID:        123,
						Name:      "Test Job",
						Status:    models.JobStatusQueued,
						Labels:    []string{"ubuntu-latest"},
						CreatedAt: now,
						RunID:     456,
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedTime: time.Date(2023, 10, 15, 14, 30, 45, 0, time.UTC),
			expectError:  false,
		},
		{
			name:           "empty data",
			eventData:      []byte(""),
			expectedTime:   time.Time{},
			expectError:    true,
			errorSubstring: "failed to parse workflow_job JSON payload",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timestamp, err := handler.ExtractEventTimestamp(tc.eventData)

			if tc.expectError {
				assert.Error(t, err, "Expected an error for test case: %s", tc.name)
				if tc.errorSubstring != "" {
					assert.Contains(t, err.Error(), tc.errorSubstring, "Error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tc.name)
				assert.Equal(t, tc.expectedTime.Unix(), timestamp.Unix(), "Timestamp should match expected value")
			}
		})
	}
}

func TestWorkflowJobHandler_ExtractOrderingKey(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	testCases := []struct {
		name           string
		eventData      []byte
		expectedKey    string
		expectError    bool
		errorSubstring string
	}{
		{
			name: "valid event with job ID",
			eventData: func() []byte {
				event := models.WorkflowJobEvent{
					Action: "queued",
					WorkflowJob: models.WorkflowJob{
						ID:        12345,
						Name:      "Test Job",
						Status:    models.JobStatusQueued,
						Labels:    []string{"ubuntu-latest"},
						CreatedAt: time.Now(),
						RunID:     67890,
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedKey: "job_12345",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key, err := handler.ExtractOrderingKey(tc.eventData)

			if tc.expectError {
				assert.Error(t, err, "Expected an error for test case: %s", tc.name)
				if tc.errorSubstring != "" {
					assert.Contains(t, err.Error(), tc.errorSubstring, "Error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tc.name)
				assert.Equal(t, tc.expectedKey, key, "Ordering key should match expected value")
			}
		})
	}
}

func TestWorkflowJobHandler_GetStatusPriority(t *testing.T) {
	mockDB, testConfig := setupWorkflowJobTest()
	handler := NewWorkflowJobHandler(testConfig, mockDB)

	testCases := []struct {
		name             string
		eventData        []byte
		expectedPriority int
		expectError      bool
		errorSubstring   string
	}{
		{
			name: "waiting status",
			eventData: func() []byte {
				event := models.WorkflowJobEvent{
					Action: "waiting",
					WorkflowJob: models.WorkflowJob{
						ID:        123,
						Name:      "Test Job",
						Status:    models.JobStatusWaiting,
						Labels:    []string{"ubuntu-latest"},
						CreatedAt: time.Now(),
						RunID:     456,
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 1,
			expectError:      false,
		},
		{
			name: "queued status",
			eventData: func() []byte {
				event := models.WorkflowJobEvent{
					Action: "queued",
					WorkflowJob: models.WorkflowJob{
						ID:        123,
						Name:      "Test Job",
						Status:    models.JobStatusQueued,
						Labels:    []string{"ubuntu-latest"},
						CreatedAt: time.Now(),
						RunID:     456,
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 2,
			expectError:      false,
		},
		{
			name: "requested status",
			eventData: func() []byte {
				event := models.WorkflowJobEvent{
					Action: "requested",
					WorkflowJob: models.WorkflowJob{
						ID:        123,
						Name:      "Test Job",
						Status:    models.JobStatusRequested,
						Labels:    []string{"ubuntu-latest"},
						CreatedAt: time.Now(),
						RunID:     456,
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 3,
			expectError:      false,
		},
		{
			name: "in_progress status",
			eventData: func() []byte {
				event := models.WorkflowJobEvent{
					Action: "in_progress",
					WorkflowJob: models.WorkflowJob{
						ID:        123,
						Name:      "Test Job",
						Status:    models.JobStatusInProgress,
						Labels:    []string{"ubuntu-latest"},
						CreatedAt: time.Now(),
						RunID:     456,
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 4,
			expectError:      false,
		},
		{
			name: "completed status",
			eventData: func() []byte {
				event := models.WorkflowJobEvent{
					Action: "completed",
					WorkflowJob: models.WorkflowJob{
						ID:        123,
						Name:      "Test Job",
						Status:    models.JobStatusCompleted,
						Labels:    []string{"ubuntu-latest"},
						CreatedAt: time.Now(),
						RunID:     456,
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 5,
			expectError:      false,
		},
		{
			name: "cancelled status",
			eventData: func() []byte {
				event := models.WorkflowJobEvent{
					Action: "cancelled",
					WorkflowJob: models.WorkflowJob{
						ID:        123,
						Name:      "Test Job",
						Status:    models.JobStatusCancelled,
						Labels:    []string{"ubuntu-latest"},
						CreatedAt: time.Now(),
						RunID:     456,
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 5,
			expectError:      false,
		},
		{
			name: "unknown status",
			eventData: func() []byte {
				event := models.WorkflowJobEvent{
					Action: "unknown_action",
					WorkflowJob: models.WorkflowJob{
						ID:        123,
						Name:      "Test Job",
						Status:    models.JobStatusQueued, // Status field doesn't matter, action is used
						Labels:    []string{"ubuntu-latest"},
						CreatedAt: time.Now(),
						RunID:     456,
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 999, // Default for unknown status
			expectError:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			priority, err := handler.GetStatusPriority(tc.eventData)

			if tc.expectError {
				assert.Error(t, err, "Expected an error for test case: %s", tc.name)
				if tc.errorSubstring != "" {
					assert.Contains(t, err.Error(), tc.errorSubstring, "Error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", tc.name)
				assert.Equal(t, tc.expectedPriority, priority, "Priority should match expected value")
			}
		})
	}
}
