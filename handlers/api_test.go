package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/internal/utils"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockPrometheusService struct {
	mock.Mock
}

func (m *MockPrometheusService) GetMetricsWithTimeSeries(period, start, end, step string) (*models.MetricsResponse, error) {
	args := m.Called(period, start, end, step)
	return args.Get(0).(*models.MetricsResponse), args.Error(1)
}

func (m *MockPrometheusService) QueryPrometheus(path string, queryParams url.Values) ([]byte, error) {
	args := m.Called(path, queryParams)
	return args.Get(0).([]byte), args.Error(1)
}

func setupAPITest() (*gin.Engine, *database.MockDatabase, *config.Config, *MockPrometheusService) {
	// Initialize logger for tests
	logger.InitLogger("error")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	mockDB := &database.MockDatabase{}
	mockPrometheus := &MockPrometheusService{}

	// Create test config
	testConfig := &config.Config{
		Vars: config.Vars{
			PrometheusURL: "http://localhost:9090",
		},
	}

	return router, mockDB, testConfig, mockPrometheus
}

func TestNewAPIHandler(t *testing.T) {
	_, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	assert.NotNil(t, handler, "NewAPIHandler should return a non-nil handler")
	assert.Equal(t, mockDB, handler.db, "Handler should store the database interface")
}

func TestValidateOrigin_MissingReferer(t *testing.T) {
	router, _, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Missing referer header")
}

func TestValidateOrigin_InvalidReferer(t *testing.T) {
	router, _, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Referer", "://invalid-url")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid referer")
}

func TestValidateOrigin_WrongHost(t *testing.T) {
	router, _, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://evil.com:8080/dashboard")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "can only be accessed from the local dashboard")
}

func TestValidateOrigin_WrongPath(t *testing.T) {
	router, _, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/wrong-path")
	router.ServeHTTP(w, req)

	// Same host passes the origin check but fails on missing CSRF cookie
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid CSRF cookie")
}

func TestValidateOrigin_MissingCSRFCookie(t *testing.T) {
	router, _, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/dashboard")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid CSRF cookie")
}

func TestValidateOrigin_MissingCSRFHeader(t *testing.T) {
	router, _, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/dashboard")
	req.AddCookie(&http.Cookie{
		Name:  utils.CookieName,
		Value: "test-token",
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid CSRF token")
}

func TestValidateOrigin_MismatchedCSRFToken(t *testing.T) {
	router, _, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/dashboard")
	req.Header.Set(utils.HeaderName, "wrong-token")
	req.AddCookie(&http.Cookie{
		Name:  utils.CookieName,
		Value: "test-token",
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid CSRF token")
}

func TestValidateOrigin_Success(t *testing.T) {
	router, _, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/dashboard")
	req.Header.Set(utils.HeaderName, "test-token")
	req.AddCookie(&http.Cookie{
		Name:  utils.CookieName,
		Value: "test-token",
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestGetWorkflowJobsByRunID_Success(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Mock data
	expectedJobs := []models.WorkflowJob{
		{
			ID:          1,
			Name:        "Test Job",
			Status:      models.JobStatusCompleted,
			Labels:      []string{"self-hosted", "ubuntu-latest"},
			Conclusion:  "success",
			CreatedAt:   time.Now().Add(-10 * time.Minute),
			StartedAt:   time.Now(),
			CompletedAt: time.Now().Add(5 * time.Minute),
			RunID:       1,
			RunnerType:  "self-hosted",
		},
	}
	mockDB.On("GetWorkflowJobsByRunID", int64(1)).Return(expectedJobs, nil)

	router.GET("/api/workflow-jobs/:run_id", handler.GetWorkflowJobsByRunID())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-jobs/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	mockDB.AssertExpectations(t)
}

func TestGetWorkflowJobsByRunID_DatabaseError(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	mockDB.On("GetWorkflowJobsByRunID", int64(1)).Return([]models.WorkflowJob{}, errors.New("database error"))

	router.GET("/api/workflow-jobs/:run_id", handler.GetWorkflowJobsByRunID())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-jobs/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to retrieve workflow jobs")

	mockDB.AssertExpectations(t)
}

func TestGetWorkflowRuns_Success(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Mock data
	now := time.Now()
	expectedRuns := []models.WorkflowRun{
		{
			ID:             1,
			Name:           "Test Workflow",
			Status:         models.JobStatusCompleted,
			RepositoryName: "test/repo",
			HtmlUrl:        "https://github.com/test/repo/actions/runs/1",
			DisplayTitle:   "Test Workflow",
			Conclusion:     "success",
			CreatedAt:      now,
			RunStartedAt:   now,
			UpdatedAt:      now,
		},
	}

	mockDB.On("GetWorkflowRunsPaginated", 1, 25).Return(expectedRuns, 1, nil)

	router.GET("/api/workflow-runs", handler.GetWorkflowRuns())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-runs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response, "workflow_runs")
	assert.Contains(t, response, "pagination")

	pagination := response["pagination"].(map[string]interface{})
	assert.Equal(t, float64(1), pagination["current_page"])
	assert.Equal(t, float64(1), pagination["total_pages"])
	assert.Equal(t, float64(1), pagination["total_count"])
	assert.Equal(t, float64(25), pagination["page_size"])
	assert.Equal(t, false, pagination["has_next"])
	assert.Equal(t, false, pagination["has_previous"])

	mockDB.AssertExpectations(t)
}

func TestGetWorkflowRuns_WithPagination(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	expectedRuns := []models.WorkflowRun{}
	mockDB.On("GetWorkflowRunsPaginated", 2, 10).Return(expectedRuns, 50, nil)

	router.GET("/api/workflow-runs", handler.GetWorkflowRuns())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-runs?page=2&limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	pagination := response["pagination"].(map[string]interface{})
	assert.Equal(t, float64(2), pagination["current_page"])
	assert.Equal(t, float64(5), pagination["total_pages"]) // 50/10 = 5
	assert.Equal(t, float64(50), pagination["total_count"])
	assert.Equal(t, float64(10), pagination["page_size"])
	assert.Equal(t, true, pagination["has_next"])
	assert.Equal(t, true, pagination["has_previous"])

	mockDB.AssertExpectations(t)
}

func TestGetWorkflowRuns_InvalidPagination(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Should default to page=1, limit=25 for invalid values
	expectedRuns := []models.WorkflowRun{}
	mockDB.On("GetWorkflowRunsPaginated", 1, 25).Return(expectedRuns, 0, nil)

	router.GET("/api/workflow-runs", handler.GetWorkflowRuns())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-runs?page=invalid&limit=200", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	pagination := response["pagination"].(map[string]interface{})
	assert.Equal(t, float64(1), pagination["current_page"])
	assert.Equal(t, float64(25), pagination["page_size"])

	mockDB.AssertExpectations(t)
}

func TestGetWorkflowRuns_DatabaseError(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	mockDB.On("GetWorkflowRunsPaginated", 1, 25).Return([]models.WorkflowRun{}, 0, errors.New("database error"))

	router.GET("/api/workflow-runs", handler.GetWorkflowRuns())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-runs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to retrieve workflow runs")

	mockDB.AssertExpectations(t)
}

func TestGetLabelMetrics_Success(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Mock data
	expectedJobsByLabel := []models.LabelMetrics{
		{
			Labels:         []string{"self-hosted", "ubuntu-latest"},
			RunnerType:     "self-hosted",
			QueuedCount:    5,
			RunningCount:   3,
			CompletedCount: 10,
			CancelledCount: 5,
			TotalCount:     23,
		},
	}
	mockDB.On("GetJobsByLabel", 1, 25).Return(expectedJobsByLabel, 1, nil)

	router.GET("/api/label-metrics", handler.GetLabelMetrics())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/label-metrics", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response, "label_metrics")
	assert.Contains(t, response, "pagination")

	pagination := response["pagination"].(map[string]interface{})
	assert.Equal(t, float64(1), pagination["current_page"])
	assert.Equal(t, float64(1), pagination["total_pages"])
	assert.Equal(t, float64(1), pagination["total_count"])
	assert.Equal(t, float64(25), pagination["page_size"])
	assert.Equal(t, false, pagination["has_next"])
	assert.Equal(t, false, pagination["has_previous"])

	mockDB.AssertExpectations(t)
}

func TestGetLabelMetrics_DatabaseError(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	mockDB.On("GetJobsByLabel", 1, 25).Return([]models.LabelMetrics{}, 0, errors.New("database error"))

	router.GET("/api/label-metrics", handler.GetLabelMetrics())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/label-metrics", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to retrieve jobs metrics")

	mockDB.AssertExpectations(t)
}

func TestGetLabelMetrics_WithPagination(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	expectedMetrics := []models.LabelMetrics{}
	mockDB.On("GetJobsByLabel", 2, 10).Return(expectedMetrics, 50, nil)

	router.GET("/api/label-metrics", handler.GetLabelMetrics())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/label-metrics?page=2&limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	pagination := response["pagination"].(map[string]interface{})
	assert.Equal(t, float64(2), pagination["current_page"])
	assert.Equal(t, float64(5), pagination["total_pages"]) // 50/10 = 5
	assert.Equal(t, float64(50), pagination["total_count"])
	assert.Equal(t, float64(10), pagination["page_size"])
	assert.Equal(t, true, pagination["has_next"])
	assert.Equal(t, true, pagination["has_previous"])

	mockDB.AssertExpectations(t)
}

func TestGetLabelMetrics_InvalidPagination(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Should default to page=1, limit=25 for invalid values
	expectedMetrics := []models.LabelMetrics{}
	mockDB.On("GetJobsByLabel", 1, 25).Return(expectedMetrics, 0, nil)

	router.GET("/api/label-metrics", handler.GetLabelMetrics())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/label-metrics?page=invalid&limit=200", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	pagination := response["pagination"].(map[string]interface{})
	assert.Equal(t, float64(1), pagination["current_page"])
	assert.Equal(t, float64(25), pagination["page_size"])

	mockDB.AssertExpectations(t)
}

func TestGetCurrentMetrics_Success(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Mock successful response from prometheus service
	expectedResponse := &models.MetricsResponse{
		CurrentMetrics: map[string]float64{
			"running_jobs": 5,
			"queued_jobs":  3,
		},
		TimeSeries: struct {
			RunningJobs models.TimeSeriesData `json:"running_jobs"`
			QueuedJobs  models.TimeSeriesData `json:"queued_jobs"`
		}{
			RunningJobs: models.TimeSeriesData{
				Status: "success",
				Data: models.TimeSeriesDataInner{
					ResultType: "matrix",
					Result:     []models.TimeSeriesEntry{},
				},
			},
			QueuedJobs: models.TimeSeriesData{
				Status: "success",
				Data: models.TimeSeriesDataInner{
					ResultType: "matrix",
					Result:     []models.TimeSeriesEntry{},
				},
			},
		},
	}

	mockPrometheus.On("GetMetricsWithTimeSeries", "day", "", "", "").Return(expectedResponse, nil)

	router.GET("/api/current-metrics", handler.GetCurrentMetrics())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/current-metrics?period=day", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response, "current_metrics")
	assert.Contains(t, response, "time_series")

	currentMetrics := response["current_metrics"].(map[string]interface{})
	assert.Equal(t, float64(5), currentMetrics["running_jobs"])
	assert.Equal(t, float64(3), currentMetrics["queued_jobs"])

	mockPrometheus.AssertExpectations(t)
}

func TestGetCurrentMetrics_PrometheusError(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Mock prometheus service error
	mockPrometheus.On("GetMetricsWithTimeSeries", "day", "", "", "").Return((*models.MetricsResponse)(nil), errors.New("prometheus connection failed"))

	router.GET("/api/current-metrics", handler.GetCurrentMetrics())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/current-metrics?period=day", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to retrieve metrics")

	mockPrometheus.AssertExpectations(t)
}

func TestGetCurrentMetrics_WithTimeSeries(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Mock successful response with time series data
	expectedResponse := &models.MetricsResponse{
		CurrentMetrics: map[string]float64{
			"running_jobs": 2,
			"queued_jobs":  1,
		},
		TimeSeries: struct {
			RunningJobs models.TimeSeriesData `json:"running_jobs"`
			QueuedJobs  models.TimeSeriesData `json:"queued_jobs"`
		}{
			RunningJobs: models.TimeSeriesData{
				Status: "success",
				Data: models.TimeSeriesDataInner{
					ResultType: "matrix",
					Result:     []models.TimeSeriesEntry{},
				},
			},
			QueuedJobs: models.TimeSeriesData{
				Status: "success",
				Data: models.TimeSeriesDataInner{
					ResultType: "matrix",
					Result:     []models.TimeSeriesEntry{},
				},
			},
		},
	}

	mockPrometheus.On("GetMetricsWithTimeSeries", "hour", "2023-01-01T00:00:00Z", "2023-01-01T01:00:00Z", "60s").Return(expectedResponse, nil)

	router.GET("/api/current-metrics", handler.GetCurrentMetrics())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/current-metrics?period=hour&start=2023-01-01T00:00:00Z&end=2023-01-01T01:00:00Z&step=60s", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response, "current_metrics")
	assert.Contains(t, response, "time_series")

	currentMetrics := response["current_metrics"].(map[string]interface{})
	assert.Equal(t, float64(2), currentMetrics["running_jobs"])
	assert.Equal(t, float64(1), currentMetrics["queued_jobs"])

	mockPrometheus.AssertExpectations(t)
}

func TestGetWorkflowJobsByRunID_InvalidRunID(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	router.GET("/api/workflow-jobs/:run_id", handler.GetWorkflowJobsByRunID())

	// Test with invalid run_id format
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-jobs/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid run_id format")

	mockDB.AssertExpectations(t)
}

func TestGetWorkflowJobsByRunID_NoJobsFound(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Mock empty result from database
	mockDB.On("GetWorkflowJobsByRunID", int64(1)).Return([]models.WorkflowJob{}, nil)

	router.GET("/api/workflow-jobs/:run_id", handler.GetWorkflowJobsByRunID())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-jobs/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "No workflow jobs found for this run ID")

	mockDB.AssertExpectations(t)
}

// Integration test for the ValidateOrigin middleware with GetWorkflowRuns
func TestIntegration_ValidateOriginWithGetWorkflowRuns(t *testing.T) {
	router, mockDB, testConfig, mockPrometheus := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

	// Setup route with middleware
	router.Use(ValidateOrigin())
	router.GET("/api/workflow-runs", handler.GetWorkflowRuns())

	// Mock successful database call
	expectedRuns := []models.WorkflowRun{}
	mockDB.On("GetWorkflowRunsPaginated", 1, 25).Return(expectedRuns, 0, nil)

	// Test with valid CSRF and referer
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-runs", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/dashboard")
	req.Header.Set(utils.HeaderName, "test-token")
	req.AddCookie(&http.Cookie{
		Name:  utils.CookieName,
		Value: "test-token",
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "workflow_runs")
	assert.Contains(t, w.Body.String(), "pagination")

	mockDB.AssertExpectations(t)
}

// Test edge cases for pagination parameters
func TestGetWorkflowRuns_PaginationEdgeCases(t *testing.T) {
	testCases := []struct {
		name          string
		queryParams   string
		expectedPage  int
		expectedLimit int
	}{
		{
			name:          "negative page",
			queryParams:   "?page=-1&limit=10",
			expectedPage:  1,
			expectedLimit: 10,
		},
		{
			name:          "zero page",
			queryParams:   "?page=0&limit=10",
			expectedPage:  1,
			expectedLimit: 10,
		},
		{
			name:          "negative limit",
			queryParams:   "?page=1&limit=-5",
			expectedPage:  1,
			expectedLimit: 25,
		},
		{
			name:          "zero limit",
			queryParams:   "?page=1&limit=0",
			expectedPage:  1,
			expectedLimit: 25,
		},
		{
			name:          "limit too high",
			queryParams:   "?page=1&limit=500",
			expectedPage:  1,
			expectedLimit: 25,
		},
		{
			name:          "floating point page",
			queryParams:   "?page=1.5&limit=10",
			expectedPage:  1,
			expectedLimit: 10,
		},
		{
			name:          "empty values",
			queryParams:   "?page=&limit=",
			expectedPage:  1,
			expectedLimit: 25,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, mockDB, testConfig, mockPrometheus := setupAPITest()
			handler := NewAPIHandler(testConfig, mockDB, mockPrometheus)

			expectedRuns := []models.WorkflowRun{}
			mockDB.On("GetWorkflowRunsPaginated", tc.expectedPage, tc.expectedLimit).Return(expectedRuns, 0, nil)

			router.GET("/api/workflow-runs", handler.GetWorkflowRuns())

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/workflow-runs"+tc.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			pagination := response["pagination"].(map[string]interface{})
			assert.Equal(t, float64(tc.expectedPage), pagination["current_page"])
			assert.Equal(t, float64(tc.expectedLimit), pagination["page_size"])

			mockDB.AssertExpectations(t)
		})
	}
}
