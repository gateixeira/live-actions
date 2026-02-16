package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/utils"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGauge implements prometheus.Gauge for testing
type MockGauge struct {
	mock.Mock
	value float64
}

func (m *MockGauge) Set(value float64) {
	m.value = value
	m.Called(value)
}

func (m *MockGauge) Inc() {
	m.value++
	m.Called()
}

func (m *MockGauge) Dec() {
	m.value--
	m.Called()
}

func (m *MockGauge) Add(value float64) {
	m.value += value
	m.Called(value)
}

func (m *MockGauge) Sub(value float64) {
	m.value -= value
	m.Called(value)
}

func (m *MockGauge) SetToCurrentTime() {
	m.value = float64(time.Now().Unix())
	m.Called()
}

func (m *MockGauge) Write(metric *dto.Metric) error {
	args := m.Called(metric)
	if args.Error(0) != nil {
		return args.Error(0)
	}

	// Set the gauge value in the metric
	metric.Gauge = &dto.Gauge{
		Value: &m.value,
	}
	return nil
}

func (m *MockGauge) Desc() *prometheus.Desc {
	args := m.Called()
	return args.Get(0).(*prometheus.Desc)
}

func (m *MockGauge) Collect(ch chan<- prometheus.Metric) {
	m.Called(ch)
}

func (m *MockGauge) Describe(ch chan<- *prometheus.Desc) {
	m.Called(ch)
}

// MockGaugeVec implements prometheus.GaugeVec for testing
type MockGaugeVec struct {
	mock.Mock
	gauges map[string]*MockGauge
}

func NewMockGaugeVec() *MockGaugeVec {
	return &MockGaugeVec{
		gauges: make(map[string]*MockGauge),
	}
}

func (m *MockGaugeVec) WithLabelValues(labelValues ...string) prometheus.Gauge {
	key := ""
	for _, v := range labelValues {
		key += v
	}

	if gauge, exists := m.gauges[key]; exists {
		return gauge
	}

	gauge := &MockGauge{}
	m.gauges[key] = gauge
	args := m.Called(labelValues)
	return args.Get(0).(prometheus.Gauge)
}

func (m *MockGaugeVec) With(labels prometheus.Labels) prometheus.Gauge {
	args := m.Called(labels)
	return args.Get(0).(prometheus.Gauge)
}

func (m *MockGaugeVec) CurryWith(labels prometheus.Labels) (*prometheus.GaugeVec, error) {
	args := m.Called(labels)
	return args.Get(0).(*prometheus.GaugeVec), args.Error(1)
}

func (m *MockGaugeVec) GetMetricWithLabelValues(labelValues ...string) (prometheus.Gauge, error) {
	args := m.Called(labelValues)
	return args.Get(0).(prometheus.Gauge), args.Error(1)
}

func (m *MockGaugeVec) GetMetricWith(labels prometheus.Labels) (prometheus.Gauge, error) {
	args := m.Called(labels)
	return args.Get(0).(prometheus.Gauge), args.Error(1)
}

func (m *MockGaugeVec) DeleteLabelValues(labelValues ...string) bool {
	args := m.Called(labelValues)
	return args.Bool(0)
}

func (m *MockGaugeVec) Delete(labels prometheus.Labels) bool {
	args := m.Called(labels)
	return args.Bool(0)
}

func (m *MockGaugeVec) Reset() {
	m.Called()
}

func (m *MockGaugeVec) Collect(ch chan<- prometheus.Metric) {
	m.Called(ch)
}

func (m *MockGaugeVec) Describe(ch chan<- *prometheus.Desc) {
	m.Called(ch)
}

// NewTestConfig creates a config for testing with specified settings
func NewTestConfig(isHTTPS, isProduction bool) *config.Config {
	env := "development"
	if isProduction {
		env = "production"
	}

	return &config.Config{
		Vars: config.Vars{
			TLSEnabled:  isHTTPS,
			Environment: env,
		},
	}
}

func setupDashboardTest() (*gin.Engine, *config.Config, fstest.MapFS) {
	// Initialize logger for tests
	logger.InitLogger("error")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	testConfig := NewTestConfig(false, false) // Default to development/HTTP

	testFS := fstest.MapFS{
		"frontend/dist/index.html": &fstest.MapFile{
			Data: []byte("<!DOCTYPE html><html><head></head><body>test</body></html>"),
		},
	}

	return router, testConfig, testFS
}

func TestNewDashboardHandler(t *testing.T) {
	testConfig := NewTestConfig(false, false)
	testFS := fstest.MapFS{
		"frontend/dist/index.html": &fstest.MapFile{Data: []byte("<html><head></head></html>")},
	}
	handler := NewDashboardHandler(testConfig, testFS)

	assert.NotNil(t, handler, "NewDashboardHandler should return a non-nil handler")
	assert.Equal(t, testConfig, handler.config, "Handler should store the config")
}

func TestValidateDashboardOrigin_MissingReferer(t *testing.T) {
	router, _, _ := setupDashboardTest()
	router.Use(ValidateDashboardOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Missing referer header")
}

func TestValidateDashboardOrigin_InvalidReferer(t *testing.T) {
	router, _, _ := setupDashboardTest()
	router.Use(ValidateDashboardOrigin())
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

func TestValidateDashboardOrigin_WrongHost(t *testing.T) {
	router, _, _ := setupDashboardTest()
	router.Use(ValidateDashboardOrigin())
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

func TestValidateDashboardOrigin_WrongPath(t *testing.T) {
	router, _, _ := setupDashboardTest()
	router.Use(ValidateDashboardOrigin())
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

func TestValidateDashboardOrigin_MissingCSRFCookie(t *testing.T) {
	router, _, _ := setupDashboardTest()
	router.Use(ValidateDashboardOrigin())
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

func TestValidateDashboardOrigin_MissingCSRFHeader(t *testing.T) {
	router, _, _ := setupDashboardTest()
	router.Use(ValidateDashboardOrigin())
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

func TestValidateDashboardOrigin_MismatchedCSRFToken(t *testing.T) {
	router, _, _ := setupDashboardTest()
	router.Use(ValidateDashboardOrigin())
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

func TestValidateDashboardOrigin_Success(t *testing.T) {
	router, _, _ := setupDashboardTest()
	router.Use(ValidateDashboardOrigin())
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

func TestDashboard_CSRFTokenGeneration(t *testing.T) {
	router, testConfig, testFS := setupDashboardTest()
	handler := NewDashboardHandler(testConfig, testFS)

	router.GET("/dashboard", handler.Dashboard())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check that CSRF cookie was set
	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == utils.CookieName {
			csrfCookie = cookie
			break
		}
	}

	assert.NotNil(t, csrfCookie, "CSRF cookie should be set")
	assert.NotEmpty(t, csrfCookie.Value, "CSRF cookie should have a value")
	assert.Equal(t, "/", csrfCookie.Path, "CSRF cookie path should be '/'")
	assert.True(t, csrfCookie.HttpOnly, "CSRF cookie should be HTTP only")
	assert.False(t, csrfCookie.Secure, "CSRF cookie should not be secure in development")

	// Check cache control headers
	assert.Equal(t, "no-store, must-revalidate", w.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", w.Header().Get("Pragma"))
	assert.Equal(t, "0", w.Header().Get("Expires"))
}

func TestDashboard_SecureCookieInProduction(t *testing.T) {
	router, _, testFS := setupDashboardTest()
	testConfig := NewTestConfig(false, true) // Production environment
	handler := NewDashboardHandler(testConfig, testFS)

	router.GET("/dashboard", handler.Dashboard())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check that CSRF cookie was set as secure in production
	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == utils.CookieName {
			csrfCookie = cookie
			break
		}
	}

	assert.NotNil(t, csrfCookie, "CSRF cookie should be set")
	assert.True(t, csrfCookie.Secure, "CSRF cookie should be secure in production")
}

func TestDashboard_SecureCookieWithHTTPS(t *testing.T) {
	router, _, testFS := setupDashboardTest()
	testConfig := NewTestConfig(true, false) // HTTPS enabled
	handler := NewDashboardHandler(testConfig, testFS)

	router.GET("/dashboard", handler.Dashboard())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check that CSRF cookie was set as secure with HTTPS
	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == utils.CookieName {
			csrfCookie = cookie
			break
		}
	}

	assert.NotNil(t, csrfCookie, "CSRF cookie should be set")
	assert.True(t, csrfCookie.Secure, "CSRF cookie should be secure with HTTPS")
}

func TestDashboard_CookieExpiry(t *testing.T) {
	router, testConfig, testFS := setupDashboardTest()
	handler := NewDashboardHandler(testConfig, testFS)

	router.GET("/dashboard", handler.Dashboard())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check cookie expiry time (12 hours)
	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == utils.CookieName {
			csrfCookie = cookie
			break
		}
	}

	assert.NotNil(t, csrfCookie, "CSRF cookie should be set")
	expectedMaxAge := int(12 * time.Hour.Seconds())
	assert.Equal(t, expectedMaxAge, csrfCookie.MaxAge, "CSRF cookie should have 12 hour expiry")
}

func TestDashboard_TemplateDataStructure(t *testing.T) {
	// Test that the dashboard handler serves the React SPA correctly
	router, testConfig, testFS := setupDashboardTest()
	handler := NewDashboardHandler(testConfig, testFS)

	router.GET("/dashboard", handler.Dashboard())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Integration test for the ValidateDashboardOrigin middleware with Dashboard handler
func TestIntegration_ValidateDashboardOriginWithDashboard(t *testing.T) {
	router, testConfig, testFS := setupDashboardTest()
	handler := NewDashboardHandler(testConfig, testFS)

	// Setup route with middleware
	router.Use(ValidateDashboardOrigin())
	router.GET("/dashboard", handler.Dashboard())

	// Test with valid CSRF and referer
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/dashboard")
	req.Header.Set(utils.HeaderName, "test-token")
	req.AddCookie(&http.Cookie{
		Name:  utils.CookieName,
		Value: "test-token",
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Should have both the original CSRF cookie and a new one
	cookies := w.Result().Cookies()
	assert.GreaterOrEqual(t, len(cookies), 1, "Should have at least one cookie set")
}

// Test config interface behavior
func TestDashboardHandler_ConfigIntegration(t *testing.T) {
	testCases := []struct {
		name         string
		isHTTPS      bool
		isProduction bool
		expectSecure bool
	}{
		{
			name:         "development with HTTP",
			isHTTPS:      false,
			isProduction: false,
			expectSecure: false,
		},
		{
			name:         "development with HTTPS",
			isHTTPS:      true,
			isProduction: false,
			expectSecure: true,
		},
		{
			name:         "production with HTTP",
			isHTTPS:      false,
			isProduction: true,
			expectSecure: true,
		},
		{
			name:         "production with HTTPS",
			isHTTPS:      true,
			isProduction: true,
			expectSecure: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, _, testFS := setupDashboardTest()
			testConfig := NewTestConfig(tc.isHTTPS, tc.isProduction)
			handler := NewDashboardHandler(testConfig, testFS)

			router.GET("/dashboard", handler.Dashboard())

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/dashboard", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			// Check cookie security setting
			cookies := w.Result().Cookies()
			var csrfCookie *http.Cookie
			for _, cookie := range cookies {
				if cookie.Name == utils.CookieName {
					csrfCookie = cookie
					break
				}
			}

			assert.NotNil(t, csrfCookie, "CSRF cookie should be set")
			assert.Equal(t, tc.expectSecure, csrfCookie.Secure,
				"Cookie secure flag should match expected value for %s", tc.name)
		})
	}
}

func TestDashboard_WithRealConfig(t *testing.T) {
	// Test with a real config struct instead of mock
	realConfig := &config.Config{
		Vars: config.Vars{
			TLSEnabled:  false,
			Environment: "development",
		},
	}

	router, _, testFS := setupDashboardTest()
	handler := NewDashboardHandler(realConfig, testFS)

	router.GET("/dashboard", handler.Dashboard())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check that cookie is not secure in development
	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == utils.CookieName {
			csrfCookie = cookie
			break
		}
	}

	assert.NotNil(t, csrfCookie, "CSRF cookie should be set")
	assert.False(t, csrfCookie.Secure, "Cookie should not be secure in development")
}

// Test SameSite cookie attribute
func TestDashboard_SameSiteCookie(t *testing.T) {
	router, testConfig, testFS := setupDashboardTest()
	handler := NewDashboardHandler(testConfig, testFS)

	router.GET("/dashboard", handler.Dashboard())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check SameSite attribute (this would be in the Set-Cookie header)
	setCookieHeader := w.Header().Get("Set-Cookie")
	assert.Contains(t, setCookieHeader, "SameSite=Strict", "Cookie should have SameSite=Strict")
}
