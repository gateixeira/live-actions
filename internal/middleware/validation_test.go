package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInputValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    string
		headers        map[string]string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Valid query parameters",
			queryParams:    "?period=hour&limit=10&offset=0",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid period parameter",
			queryParams:    "?period=invalid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid query parameter",
		},
		{
			name:           "Invalid time format",
			queryParams:    "?start_time=invalid-time",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid query parameter",
		},
		{
			name:           "Valid time format - RFC3339",
			queryParams:    "?start_time=2023-01-01T00:00:00Z",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid time format - date only",
			queryParams:    "?start_time=2023-01-01",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid GitHub repo name",
			queryParams:    "?repo=my-awesome-repo&owner=github-user",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid status parameter",
			queryParams:    "?status=completed",
			expectedStatus: http.StatusOK,
		},
		{
			name:        "Valid User-Agent header",
			queryParams: "",
			headers: map[string]string{
				"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(InputValidator())
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req := httptest.NewRequest("GET", "/test"+tt.queryParams, nil)

			// Set headers if provided
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestValidateTimeString(t *testing.T) {
	tests := []struct {
		name        string
		timeString  string
		expectError bool
	}{
		{"Valid RFC3339", "2023-01-01T00:00:00Z", false},
		{"Valid date only", "2023-01-01", false},
		{"Valid datetime", "2023-01-01 12:30:45", false},
		{"Invalid format", "not-a-time", true},
		{"Empty string", "", true},
		{"Invalid month", "2023-13-01", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimeString(tt.timeString)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
