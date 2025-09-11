package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})

	tests := []struct {
		name           string
		expectedHeader string
		expectedValue  string
	}{
		{
			name:           "X-Frame-Options header",
			expectedHeader: "X-Frame-Options",
			expectedValue:  "DENY",
		},
		{
			name:           "X-Content-Type-Options header",
			expectedHeader: "X-Content-Type-Options",
			expectedValue:  "nosniff",
		},
		{
			name:           "X-XSS-Protection header",
			expectedHeader: "X-XSS-Protection",
			expectedValue:  "1; mode=block",
		},
		{
			name:           "Strict-Transport-Security header",
			expectedHeader: "Strict-Transport-Security",
			expectedValue:  "max-age=31536000; includeSubDomains",
		},
		{
			name:           "Referrer-Policy header",
			expectedHeader: "Referrer-Policy",
			expectedValue:  "strict-origin-when-cross-origin",
		},
		{
			name:           "Content-Security-Policy header",
			expectedHeader: "Content-Security-Policy",
			expectedValue: "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
				"style-src 'self' 'unsafe-inline' https://unpkg.com; img-src 'self' data:; " +
				"font-src 'self' https:; connect-src 'self'; frame-ancestors 'none'; " +
				"base-uri 'self'; form-action 'self'",
		},
		{
			name:           "Permissions-Policy header",
			expectedHeader: "Permissions-Policy",
			expectedValue:  "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), ambient-light-sensor=()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req, _ := http.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			// Execute request
			router.ServeHTTP(w, req)

			// Assert status code
			assert.Equal(t, http.StatusOK, w.Code)

			// Assert security header is present and has correct value
			headerValue := w.Header().Get(tt.expectedHeader)
			assert.Equal(t, tt.expectedValue, headerValue, "Header %s should have correct value", tt.expectedHeader)
		})
	}
}

func TestSecurityHeaders_AllHeadersPresent(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})

	// Create request
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Assert all security headers are present
	expectedHeaders := []string{
		"X-Frame-Options",
		"X-Content-Type-Options",
		"X-XSS-Protection",
		"Strict-Transport-Security",
		"Referrer-Policy",
		"Content-Security-Policy",
		"Permissions-Policy",
	}

	for _, header := range expectedHeaders {
		assert.NotEmpty(t, w.Header().Get(header), "Header %s should be present", header)
	}
}

func TestSecurityHeaders_NoInterferenceWithResponse(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test", "data": "response"})
	})

	// Create request
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Assert status code is correct
	assert.Equal(t, http.StatusOK, w.Code)

	// Assert response body is not affected
	assert.Contains(t, w.Body.String(), "test")
	assert.Contains(t, w.Body.String(), "response")

	// Assert Content-Type is still set properly
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestSecurityHeaders_MultipleRequests(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test1", func(c *gin.Context) {
		c.JSON(200, gin.H{"endpoint": "test1"})
	})
	router.POST("/test2", func(c *gin.Context) {
		c.JSON(201, gin.H{"endpoint": "test2"})
	})

	tests := []struct {
		method     string
		path       string
		statusCode int
	}{
		{"GET", "/test1", 200},
		{"POST", "/test2", 201},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Assert status code
			assert.Equal(t, tt.statusCode, w.Code)

			// Assert security headers are still present
			assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
		})
	}
}

func TestSecurityHeaders_CSPConfiguration(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})

	// Create request
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")

	// Verify CSP contains expected directives
	expectedDirectives := []string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net",
		"style-src 'self' 'unsafe-inline' https://unpkg.com",
		"img-src 'self' data:",
		"font-src 'self' https:",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}

	for _, directive := range expectedDirectives {
		assert.Contains(t, csp, directive, "CSP should contain directive: %s", directive)
	}
}
