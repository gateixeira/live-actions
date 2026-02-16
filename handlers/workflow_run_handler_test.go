package handlers

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupWorkflowRunTest() *database.MockDatabase {
	// Initialize logger for tests
	logger.InitLogger("error")

	// Initialize SSE handler to prevent panics
	InitSSEHandler()

	return &database.MockDatabase{}
}

func TestNewWorkflowRunHandler(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	assert.NotNil(t, handler, "NewWorkflowRunHandler should return a non-nil handler")
	assert.Equal(t, mockDB, handler.db, "Handler should store the database interface")
}

func TestWorkflowRunHandler_GetEventType(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	eventType := handler.GetEventType()
	assert.Equal(t, "workflow_run", eventType, "GetEventType should return 'workflow_run'")
}

func TestWorkflowRunHandler_HandleEvent_Success(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	// Create test data
	now := time.Now()
	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	workflowRunEvent := models.WorkflowRunEvent{
		Action: "completed",
		Repository: models.Repository{
			Name: "test/repo",
			Url:  "https://github.com/test/repo",
		},
		WorkflowRun: models.WorkflowRun{
			ID:           12345,
			Name:         "Test Workflow",
			Status:       models.JobStatusInProgress, // This will be overridden by action
			HtmlUrl:      "https://github.com/test/repo/actions/runs/12345",
			DisplayTitle: "Test Workflow Run",
			Conclusion:   "success",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	eventData, err := json.Marshal(workflowRunEvent)
	assert.NoError(t, err, "Should be able to marshal test data")

	// Set up mock expectations - use MatchedBy for flexible comparison
	mockDB.On("AddOrUpdateRun", mock.Anything, mock.MatchedBy(func(run models.WorkflowRun) bool {
		return run.ID == 12345 &&
			run.Name == "Test Workflow" &&
			run.Status == models.JobStatus("completed") &&
			run.HtmlUrl == "https://github.com/test/repo/actions/runs/12345" &&
			run.DisplayTitle == "Test Workflow Run" &&
			run.Conclusion == "success" &&
			run.RepositoryName == "test/repo"
	}), mock.AnythingOfType("time.Time")).Return(true, nil)

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Verify results
	assert.NoError(t, err, "HandleEvent should not return an error")
	mockDB.AssertExpectations(t)
}

func TestWorkflowRunHandler_HandleEvent_InvalidJSON(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

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

func TestWorkflowRunHandler_HandleEvent_DatabaseError(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	// Create test data
	now := time.Now()
	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	workflowRunEvent := models.WorkflowRunEvent{
		Action: "requested",
		Repository: models.Repository{
			Name: "test/repo",
			Url:  "https://github.com/test/repo",
		},
		WorkflowRun: models.WorkflowRun{
			ID:           12345,
			Name:         "Test Workflow",
			Status:       models.JobStatusQueued,
			HtmlUrl:      "https://github.com/test/repo/actions/runs/12345",
			DisplayTitle: "Test Workflow Run",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	eventData, err := json.Marshal(workflowRunEvent)
	assert.NoError(t, err, "Should be able to marshal test data")

	// Set up mock expectations with database error - use MatchedBy for flexible comparison
	mockDB.On("AddOrUpdateRun", mock.Anything, mock.MatchedBy(func(run models.WorkflowRun) bool {
		return run.ID == 12345 &&
			run.Name == "Test Workflow" &&
			run.Status == models.JobStatus("requested") &&
			run.HtmlUrl == "https://github.com/test/repo/actions/runs/12345" &&
			run.DisplayTitle == "Test Workflow Run" &&
			run.RepositoryName == "test/repo"
	}), mock.AnythingOfType("time.Time")).Return(false, errors.New("database connection failed"))

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Verify results
	assert.Error(t, err, "HandleEvent should return an error when database fails")
	assert.Contains(t, err.Error(), "failed to save workflow run", "Error should mention saving workflow run")
	mockDB.AssertExpectations(t)
}

func TestWorkflowRunHandler_HandleEvent_DifferentActions(t *testing.T) {
	testCases := []struct {
		name     string
		action   string
		expected models.JobStatus
	}{
		{
			name:     "requested action",
			action:   "requested",
			expected: models.JobStatus("requested"),
		},
		{
			name:     "in_progress action",
			action:   "in_progress",
			expected: models.JobStatus("in_progress"),
		},
		{
			name:     "completed action",
			action:   "completed",
			expected: models.JobStatus("completed"),
		},
		{
			name:     "cancelled action",
			action:   "cancelled",
			expected: models.JobStatus("cancelled"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := setupWorkflowRunTest()
			handler := NewWorkflowRunHandler(mockDB)

			now := time.Now()
			sequence := &models.EventSequence{
				EventID:    "event123",
				SequenceID: 1,
				Timestamp:  now,
				DeliveryID: "delivery123",
				ReceivedAt: now,
			}

			workflowRunEvent := models.WorkflowRunEvent{
				Action: tc.action,
				Repository: models.Repository{
					Name: "test/repo",
					Url:  "https://github.com/test/repo",
				},
				WorkflowRun: models.WorkflowRun{
					ID:           12345,
					Name:         "Test Workflow",
					Status:       models.JobStatusQueued, // This will be overridden
					HtmlUrl:      "https://github.com/test/repo/actions/runs/12345",
					DisplayTitle: "Test Workflow Run",
					CreatedAt:    now,
					UpdatedAt:    now,
				},
			}

			eventData, err := json.Marshal(workflowRunEvent)
			assert.NoError(t, err, "Should be able to marshal test data")

			// Set up mock expectations - use MatchedBy for flexible comparison
			mockDB.On("AddOrUpdateRun", mock.Anything, mock.MatchedBy(func(run models.WorkflowRun) bool {
				return run.ID == 12345 &&
					run.Name == "Test Workflow" &&
					run.Status == tc.expected &&
					run.HtmlUrl == "https://github.com/test/repo/actions/runs/12345" &&
					run.DisplayTitle == "Test Workflow Run" &&
					run.RepositoryName == "test/repo"
			}), mock.AnythingOfType("time.Time")).Return(true, nil)

			// Execute the handler
			err = handler.HandleEvent(eventData, sequence)

			// Verify results
			assert.NoError(t, err, "HandleEvent should not return an error")
			mockDB.AssertExpectations(t)
		})
	}
}

func TestWorkflowRunHandler_HandleEvent_StatusAndRepositoryMapping(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	now := time.Now()
	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	workflowRunEvent := models.WorkflowRunEvent{
		Action: "in_progress",
		Repository: models.Repository{
			Name: "owner/repository-name",
			Url:  "https://github.com/owner/repository-name",
		},
		WorkflowRun: models.WorkflowRun{
			ID:           99999,
			Name:         "CI Pipeline",
			Status:       models.JobStatusQueued, // Should be overridden to "in_progress"
			HtmlUrl:      "https://github.com/owner/repository-name/actions/runs/99999",
			DisplayTitle: "CI Pipeline Run",
			Conclusion:   "",
			CreatedAt:    now,
			UpdatedAt:    now,
			// RepositoryName should be empty initially and set from Repository.Name
		},
	}

	eventData, err := json.Marshal(workflowRunEvent)
	assert.NoError(t, err, "Should be able to marshal test data")

	// Capture the argument passed to AddOrUpdateRun to verify the transformation
	var capturedRun models.WorkflowRun
	mockDB.On("AddOrUpdateRun", mock.Anything, mock.MatchedBy(func(run models.WorkflowRun) bool {
		capturedRun = run
		return true
	}), mock.AnythingOfType("time.Time")).Return(true, nil)

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Verify results
	assert.NoError(t, err, "HandleEvent should not return an error")

	// Verify the transformations
	assert.Equal(t, models.JobStatus("in_progress"), capturedRun.Status, "Status should be set from action")
	assert.Equal(t, "owner/repository-name", capturedRun.RepositoryName, "RepositoryName should be set from Repository.Name")
	assert.Equal(t, int64(99999), capturedRun.ID, "ID should be preserved")
	assert.Equal(t, "CI Pipeline", capturedRun.Name, "Name should be preserved")

	mockDB.AssertExpectations(t)
}

func TestWorkflowRunHandler_HandleEvent_EmptyEventData(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	sequence := &models.EventSequence{
		EventID:    "event123",
		DeliveryID: "delivery123",
	}

	err := handler.HandleEvent([]byte(""), sequence)

	assert.Error(t, err, "HandleEvent should return an error for empty data")
	assert.Contains(t, err.Error(), "invalid JSON payload", "Error should mention invalid JSON")
	mockDB.AssertExpectations(t) // No database calls should have been made
}

func TestWorkflowRunHandler_HandleEvent_MalformedJSON(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	sequence := &models.EventSequence{
		EventID:    "event123",
		DeliveryID: "delivery123",
	}

	malformedJSON := []byte(`{
		"action": "completed",
		"repository": {
			"name": "test/repo"
			// Missing comma
			"url": "https://github.com/test/repo"
		}
	}`)

	err := handler.HandleEvent(malformedJSON, sequence)

	assert.Error(t, err, "HandleEvent should return an error for malformed JSON")
	assert.Contains(t, err.Error(), "invalid JSON payload", "Error should mention invalid JSON")
	mockDB.AssertExpectations(t) // No database calls should have been made
}

// Test with minimal required fields
func TestWorkflowRunHandler_HandleEvent_MinimalRequiredFields(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	now := time.Now()
	sequence := &models.EventSequence{
		EventID:    "event123",
		SequenceID: 1,
		Timestamp:  now,
		DeliveryID: "delivery123",
		ReceivedAt: now,
	}

	// Create minimal workflow run event with only required fields
	workflowRunEvent := models.WorkflowRunEvent{
		Action: "completed",
		Repository: models.Repository{
			Name: "minimal/repo",
			Url:  "https://github.com/minimal/repo",
		},
		WorkflowRun: models.WorkflowRun{
			ID:           1,
			Name:         "Minimal Workflow",
			Status:       models.JobStatusQueued,
			HtmlUrl:      "https://github.com/minimal/repo/actions/runs/1",
			DisplayTitle: "Minimal Test",
			CreatedAt:    now,
		},
	}

	eventData, err := json.Marshal(workflowRunEvent)
	assert.NoError(t, err, "Should be able to marshal minimal test data")

	// Set up mock expectations - use MatchedBy for flexible comparison
	mockDB.On("AddOrUpdateRun", mock.Anything, mock.MatchedBy(func(run models.WorkflowRun) bool {
		return run.ID == 1 &&
			run.Name == "Minimal Workflow" &&
			run.Status == models.JobStatus("completed") &&
			run.HtmlUrl == "https://github.com/minimal/repo/actions/runs/1" &&
			run.DisplayTitle == "Minimal Test" &&
			run.RepositoryName == "minimal/repo"
	}), mock.AnythingOfType("time.Time")).Return(true, nil)

	// Execute the handler
	err = handler.HandleEvent(eventData, sequence)

	// Verify results
	assert.NoError(t, err, "HandleEvent should work with minimal required fields")
	mockDB.AssertExpectations(t)
}

func TestWorkflowRunHandler_ExtractEventTimestamp(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

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
				event := models.WorkflowRunEvent{
					Action: "completed",
					Repository: models.Repository{
						Name: "test/repo",
						Url:  "https://github.com/test/repo",
					},
					WorkflowRun: models.WorkflowRun{
						ID:           123,
						Name:         "Test Workflow",
						Status:       models.JobStatusCompleted,
						HtmlUrl:      "https://github.com/test/repo/actions/runs/123",
						DisplayTitle: "Test Run",
						CreatedAt:    now,
						UpdatedAt:    now,
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
			errorSubstring: "failed to parse workflow_run JSON payload",
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

func TestWorkflowRunHandler_ExtractOrderingKey(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	testCases := []struct {
		name           string
		eventData      []byte
		expectedKey    string
		expectError    bool
		errorSubstring string
	}{
		{
			name: "valid event with run ID",
			eventData: func() []byte {
				event := models.WorkflowRunEvent{
					Action: "completed",
					Repository: models.Repository{
						Name: "test/repo",
						Url:  "https://github.com/test/repo",
					},
					WorkflowRun: models.WorkflowRun{
						ID:           12345,
						Name:         "Test Workflow",
						Status:       models.JobStatusCompleted,
						HtmlUrl:      "https://github.com/test/repo/actions/runs/12345",
						DisplayTitle: "Test Run",
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedKey: "run_12345",
			expectError: false,
		},
		{
			name:           "empty data",
			eventData:      []byte(""),
			expectedKey:    "",
			expectError:    true,
			errorSubstring: "failed to parse workflow_run JSON payload",
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

func TestWorkflowRunHandler_GetStatusPriority(t *testing.T) {
	mockDB := setupWorkflowRunTest()
	handler := NewWorkflowRunHandler(mockDB)

	testCases := []struct {
		name             string
		eventData        []byte
		expectedPriority int
		expectError      bool
		errorSubstring   string
	}{
		{
			name: "requested status",
			eventData: func() []byte {
				event := models.WorkflowRunEvent{
					Action: "requested",
					Repository: models.Repository{
						Name: "test/repo",
						Url:  "https://github.com/test/repo",
					},
					WorkflowRun: models.WorkflowRun{
						ID:           123,
						Name:         "Test Workflow",
						Status:       models.JobStatusRequested,
						HtmlUrl:      "https://github.com/test/repo/actions/runs/123",
						DisplayTitle: "Test Run",
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 1,
			expectError:      false,
		},
		{
			name: "in_progress status",
			eventData: func() []byte {
				event := models.WorkflowRunEvent{
					Action: "in_progress",
					Repository: models.Repository{
						Name: "test/repo",
						Url:  "https://github.com/test/repo",
					},
					WorkflowRun: models.WorkflowRun{
						ID:           123,
						Name:         "Test Workflow",
						Status:       models.JobStatusInProgress,
						HtmlUrl:      "https://github.com/test/repo/actions/runs/123",
						DisplayTitle: "Test Run",
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 2,
			expectError:      false,
		},
		{
			name: "completed status",
			eventData: func() []byte {
				event := models.WorkflowRunEvent{
					Action: "completed",
					Repository: models.Repository{
						Name: "test/repo",
						Url:  "https://github.com/test/repo",
					},
					WorkflowRun: models.WorkflowRun{
						ID:           123,
						Name:         "Test Workflow",
						Status:       models.JobStatusCompleted,
						HtmlUrl:      "https://github.com/test/repo/actions/runs/123",
						DisplayTitle: "Test Run",
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 3,
			expectError:      false,
		},
		{
			name: "cancelled status",
			eventData: func() []byte {
				event := models.WorkflowRunEvent{
					Action: "cancelled",
					Repository: models.Repository{
						Name: "test/repo",
						Url:  "https://github.com/test/repo",
					},
					WorkflowRun: models.WorkflowRun{
						ID:           123,
						Name:         "Test Workflow",
						Status:       models.JobStatusCancelled,
						HtmlUrl:      "https://github.com/test/repo/actions/runs/123",
						DisplayTitle: "Test Run",
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 3,
			expectError:      false,
		},
		{
			name: "unknown status",
			eventData: func() []byte {
				event := models.WorkflowRunEvent{
					Action: "unknown_action",
					Repository: models.Repository{
						Name: "test/repo",
						Url:  "https://github.com/test/repo",
					},
					WorkflowRun: models.WorkflowRun{
						ID:           123,
						Name:         "Test Workflow",
						Status:       models.JobStatusCompleted, // Status field doesn't matter, action is used
						HtmlUrl:      "https://github.com/test/repo/actions/runs/123",
						DisplayTitle: "Test Run",
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			expectedPriority: 999, // Default for unknown status
			expectError:      false,
		},
		{
			name:             "empty data",
			eventData:        []byte(""),
			expectedPriority: 0,
			expectError:      true,
			errorSubstring:   "failed to parse workflow_run JSON payload",
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
