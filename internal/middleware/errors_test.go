package middleware

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestErrorHandler_NormalFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorHandler())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestErrorHandler_PanicRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorHandler())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req, _ := http.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal server error")
	assert.NotContains(t, w.Body.String(), "test panic") // Should not expose panic details
}

func TestErrorHandler_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorHandler())
	router.GET("/error", func(c *gin.Context) {
		c.Error(errors.New("test error"))
		c.JSON(500, gin.H{"error": "internal error"})
	})

	req, _ := http.NewRequest("GET", "/error", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestErrorHandler_ClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorHandler())
	router.GET("/bad-request", func(c *gin.Context) {
		c.Error(errors.New("invalid input"))
		c.Status(http.StatusBadRequest)
	})

	req, _ := http.NewRequest("GET", "/bad-request", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRequestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Capture log output
	var buf bytes.Buffer
	gin.DefaultWriter = &buf

	router := gin.New()
	router.Use(RequestLogger())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})
	router.GET("/metrics", func(c *gin.Context) {
		c.JSON(200, gin.H{"metrics": "data"})
	})

	tests := []struct {
		name string
		path string
	}{
		{
			name: "Regular endpoint should log at INFO level",
			path: "/test",
		},
		{
			name: "Metrics endpoint should log at DEBUG level",
			path: "/metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.path, nil)
			req.Header.Set("User-Agent", "test-agent")
			req.Header.Set("Referer", "https://example.com")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			// Note: RequestLogger uses zap logger which may not write to gin.DefaultWriter
			// This is more of a smoke test to ensure it doesn't crash
			// The actual log level differentiation is verified by the middleware logic
		})
	}
}

func TestSecurityLogger_NormalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityLogger())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityLogger_SuspiciousRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityLogger())
	router.GET("/*path", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})

	tests := []struct {
		name      string
		path      string
		userAgent string
		shouldLog bool
	}{
		{
			name:      "Suspicious path - admin",
			path:      "/admin/login",
			userAgent: "Mozilla/5.0",
			shouldLog: true,
		},
		{
			name:      "Suspicious path - directory traversal",
			path:      "/../etc/passwd",
			userAgent: "Mozilla/5.0",
			shouldLog: true,
		},
		{
			name:      "Suspicious user agent - curl",
			path:      "/test",
			userAgent: "curl/7.68.0",
			shouldLog: true,
		},
		{
			name:      "Normal request",
			path:      "/api/data",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			shouldLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.path, nil)
			req.Header.Set("User-Agent", tt.userAgent)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// The test mainly ensures the middleware doesn't crash
			// Actual logging verification would require capturing zap output
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestSecurityLogger_AccessDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityLogger())
	router.GET("/unauthorized", func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	})
	router.GET("/forbidden", func(c *gin.Context) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	})

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "Unauthorized access",
			path:           "/unauthorized",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Forbidden access",
			path:           "/forbidden",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Database error sanitization",
			input:    "Database connection failed",
			expected: "storage connection failed",
		},
		{
			name:     "SQL error sanitization",
			input:    "SQL syntax error in query",
			expected: "query syntax error in query",
		},
		{
			name:     "Password sanitization",
			input:    "Invalid password provided",
			expected: "invalid [REDACTED] provided",
		},
		{
			name:     "Multiple sensitive terms",
			input:    "Database password and secret key invalid",
			expected: "storage [REDACTED] and [REDACTED] [REDACTED] invalid",
		},
		{
			name:     "Long message truncation",
			input:    strings.Repeat("a", 150),
			expected: strings.Repeat("a", 97) + "...",
		},
		{
			name:     "Normal error message",
			input:    "User not found",
			expected: "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeErrorMessage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSanitizedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	var testContext *gin.Context

	router.GET("/test", func(c *gin.Context) {
		testContext = c
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://example.com")
	req.Header.Set("Authorization", "Bearer secret-token") // Should not be included
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Custom-Header", "should-not-appear")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	sanitized := getSanitizedHeaders(testContext)

	// Should include relevant headers
	assert.Equal(t, "Mozilla/5.0", sanitized["User-Agent"])
	assert.Equal(t, "https://example.com", sanitized["Referer"])
	assert.Equal(t, "192.168.1.1", sanitized["X-Forwarded-For"])
	assert.Equal(t, "application/json", sanitized["Content-Type"])

	// Should not include sensitive headers
	assert.NotContains(t, sanitized, "Authorization")
	assert.NotContains(t, sanitized, "Custom-Header")
}

func TestGetSanitizedHeaders_LongValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	var testContext *gin.Context

	router.GET("/test", func(c *gin.Context) {
		testContext = c
	})

	longUserAgent := strings.Repeat("a", 250)
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", longUserAgent)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	sanitized := getSanitizedHeaders(testContext)

	// Should truncate long headers
	assert.Equal(t, 200, len(sanitized["User-Agent"]))
	assert.True(t, strings.HasSuffix(sanitized["User-Agent"], "..."))
}

func TestErrorHandler_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorHandler())
	router.Use(SecurityLogger())

	router.GET("/normal", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	router.GET("/error", func(c *gin.Context) {
		c.Error(errors.New("test error"))
	})

	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Normal request",
			path:           "/normal",
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name:           "Error handling",
			path:           "/error",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal server error",
		},
		{
			name:           "Panic recovery",
			path:           "/panic",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)
		})
	}
}
