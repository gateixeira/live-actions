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

func setupAPITest() (*gin.Engine, *database.MockDatabase, *config.Config) {
	// Initialize logger for tests
	logger.InitLogger("error")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	mockDB := &database.MockDatabase{}

	// Create test config
	testConfig := &config.Config{
		Vars: config.Vars{},
	}

	return router, mockDB, testConfig
}

func TestNewAPIHandler(t *testing.T) {
	_, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	assert.NotNil(t, handler, "NewAPIHandler should return a non-nil handler")
	assert.Equal(t, mockDB, handler.db, "Handler should store the database interface")
}

func TestValidateOrigin_MissingReferer(t *testing.T) {
	router, _, _ := setupAPITest()
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
	router, _, _ := setupAPITest()
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
	router, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://evil.com:8080/")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "can only be accessed from the application")
}

func TestValidateOrigin_WrongPath(t *testing.T) {
	router, _, _ := setupAPITest()
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
	router, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid CSRF cookie")
}

func TestValidateOrigin_MissingCSRFHeader(t *testing.T) {
	router, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/")
	req.AddCookie(&http.Cookie{
		Name:  utils.CookieName,
		Value: "test-token",
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid CSRF token")
}

func TestValidateOrigin_MismatchedCSRFToken(t *testing.T) {
	router, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/")
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
	router, _, _ := setupAPITest()
	router.Use(ValidateOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/")
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
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

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
		},
	}
	mockDB.On("GetWorkflowJobsByRunID", mock.Anything, int64(1)).Return(expectedJobs, nil)

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
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	mockDB.On("GetWorkflowJobsByRunID", mock.Anything, int64(1)).Return([]models.WorkflowJob{}, errors.New("database error"))

	router.GET("/api/workflow-jobs/:run_id", handler.GetWorkflowJobsByRunID())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-jobs/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to retrieve workflow jobs")

	mockDB.AssertExpectations(t)
}

func TestGetWorkflowRuns_Success(t *testing.T) {
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

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

	mockDB.On("GetWorkflowRunsPaginated", mock.Anything, 1, 25).Return(expectedRuns, 1, nil)

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
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	expectedRuns := []models.WorkflowRun{}
	mockDB.On("GetWorkflowRunsPaginated", mock.Anything, 2, 10).Return(expectedRuns, 50, nil)

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
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	// Should default to page=1, limit=25 for invalid values
	expectedRuns := []models.WorkflowRun{}
	mockDB.On("GetWorkflowRunsPaginated", mock.Anything, 1, 25).Return(expectedRuns, 0, nil)

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
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	mockDB.On("GetWorkflowRunsPaginated", mock.Anything, 1, 25).Return([]models.WorkflowRun{}, 0, errors.New("database error"))

	router.GET("/api/workflow-runs", handler.GetWorkflowRuns())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-runs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to retrieve workflow runs")

	mockDB.AssertExpectations(t)
}

func TestGetCurrentMetrics_Success(t *testing.T) {
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	// Mock DB responses
	mockDB.On("GetMetricsSummary", mock.Anything, mock.Anything).Return(map[string]float64{
		"running_jobs":   5,
		"queued_jobs":    3,
		"avg_queue_time": 0,
		"peak_demand":    0,
	}, nil)
	mockDB.On("GetMetricsHistory", mock.Anything, mock.Anything).Return([]models.MetricsSnapshot{}, nil)

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

	mockDB.AssertExpectations(t)
}

func TestGetCurrentMetrics_DBError(t *testing.T) {
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	mockDB.On("GetMetricsSummary", mock.Anything, mock.Anything).Return(map[string]float64(nil), assert.AnError)

	router.GET("/api/current-metrics", handler.GetCurrentMetrics())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/current-metrics?period=day", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to retrieve metrics")

	mockDB.AssertExpectations(t)
}

func TestGetCurrentMetrics_WithTimeSeries(t *testing.T) {
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	mockDB.On("GetMetricsSummary", mock.Anything, mock.Anything).Return(map[string]float64{
		"running_jobs":   2,
		"queued_jobs":    1,
		"avg_queue_time": 0,
		"peak_demand":    0,
	}, nil)
	mockDB.On("GetMetricsHistory", mock.Anything, mock.Anything).Return([]models.MetricsSnapshot{
		{Timestamp: 1672531200, Running: 2, Queued: 1},
		{Timestamp: 1672531260, Running: 3, Queued: 0},
	}, nil)

	router.GET("/api/current-metrics", handler.GetCurrentMetrics())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/current-metrics?period=hour", nil)
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

	mockDB.AssertExpectations(t)
}

func TestGetWorkflowJobsByRunID_InvalidRunID(t *testing.T) {
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

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
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	// Mock empty result from database
	mockDB.On("GetWorkflowJobsByRunID", mock.Anything, int64(1)).Return([]models.WorkflowJob{}, nil)

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
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	// Setup route with middleware
	router.Use(ValidateOrigin())
	router.GET("/api/workflow-runs", handler.GetWorkflowRuns())

	// Mock successful database call
	expectedRuns := []models.WorkflowRun{}
	mockDB.On("GetWorkflowRunsPaginated", mock.Anything, 1, 25).Return(expectedRuns, 0, nil)

	// Test with valid CSRF and referer
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/workflow-runs", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/")
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
			router, mockDB, testConfig := setupAPITest()
			handler := NewAPIHandler(testConfig, mockDB)

			expectedRuns := []models.WorkflowRun{}
			mockDB.On("GetWorkflowRunsPaginated", mock.Anything, tc.expectedPage, tc.expectedLimit).Return(expectedRuns, 0, nil)

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

func TestGetCSRFToken_Success(t *testing.T) {
	router, mockDB, testConfig := setupAPITest()
	handler := NewAPIHandler(testConfig, mockDB)

	router.GET("/api/csrf", handler.GetCSRFToken())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/csrf", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response["token"])

	// Verify cookie is set
	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == utils.CookieName {
			csrfCookie = c
			break
		}
	}
	assert.NotNil(t, csrfCookie, "CSRF cookie should be set")
	decodedValue, _ := url.QueryUnescape(csrfCookie.Value)
	assert.Equal(t, response["token"], decodedValue)
	assert.True(t, csrfCookie.HttpOnly)
}

func TestGetCSRFToken_SecureCookie(t *testing.T) {
	router, mockDB, _ := setupAPITest()
	prodConfig := &config.Config{
		Vars: config.Vars{
			Environment: "production",
		},
	}
	handler := NewAPIHandler(prodConfig, mockDB)

	router.GET("/api/csrf", handler.GetCSRFToken())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/csrf", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == utils.CookieName {
			csrfCookie = c
			break
		}
	}
	assert.NotNil(t, csrfCookie)
	assert.True(t, csrfCookie.Secure, "Cookie should be secure in production")
}

func TestValidateSSEOrigin_MissingOriginAndReferer(t *testing.T) {
	router, _, _ := setupAPITest()
	router.Use(ValidateSSEOrigin())
	router.GET("/events", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/events", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Missing origin")
}

func TestValidateSSEOrigin_CrossOrigin(t *testing.T) {
	router, _, _ := setupAPITest()
	router.Use(ValidateSSEOrigin())
	router.GET("/events", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/events", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://evil.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Cross-origin SSE connections are not allowed")
}

func TestValidateSSEOrigin_InvalidOrigin(t *testing.T) {
	router, _, _ := setupAPITest()
	router.Use(ValidateSSEOrigin())
	router.GET("/events", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/events", nil)
	req.Header.Set("Origin", "://invalid-url")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid origin")
}

func TestValidateSSEOrigin_ValidOriginHeader(t *testing.T) {
	router, _, _ := setupAPITest()
	router.Use(ValidateSSEOrigin())
	router.GET("/events", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/events", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://localhost:8080")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidateSSEOrigin_ValidRefererHeader(t *testing.T) {
	router, _, _ := setupAPITest()
	router.Use(ValidateSSEOrigin())
	router.GET("/events", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/events", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/dashboard")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidateSSEOrigin_OriginPreferredOverReferer(t *testing.T) {
	router, _, _ := setupAPITest()
	router.Use(ValidateSSEOrigin())
	router.GET("/events", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Origin says evil.com, Referer says localhost - Origin should be checked first
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/events", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Referer", "http://localhost:8080/dashboard")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
