package handlers

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/utils"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DashboardHandler struct {
	config *config.Config
}

func NewDashboardHandler(config *config.Config) *DashboardHandler {
	return &DashboardHandler{config: config}
}

// ValidateDashboardOrigin middleware ensures requests come from the dashboard UI
func ValidateDashboardOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		referer := c.Request.Header.Get("Referer")
		if referer == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. Missing referer header.",
			})
			c.Abort()
			return
		}

		// Parse the referer URL
		refererURL, err := url.Parse(referer)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. Invalid referer.",
			})
			c.Abort()
			return
		}

		// Get the request host
		requestHost := c.Request.Host

		// Compare hostnames (ignore port to support dev proxy setups)
		refererHostname := refererURL.Hostname()
		requestHostname := requestHost
		if h, _, err := net.SplitHostPort(requestHost); err == nil {
			requestHostname = h
		}

		if refererHostname != requestHostname {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. This endpoint can only be accessed from the local dashboard.",
			})
			c.Abort()
			return
		}

		// Validate CSRF token
		csrfCookie, err := c.Cookie(utils.CookieName)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Invalid CSRF cookie",
			})
			c.Abort()
			return
		}

		csrfHeader := c.GetHeader(utils.HeaderName)
		if csrfHeader == "" || csrfHeader != csrfCookie {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Invalid CSRF token",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Dashboard serves the dashboard HTML page
func (h *DashboardHandler) Dashboard() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate CSRF token
		csrfToken, err := utils.GenerateCSRFToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate security token"})
			return
		}

		// Set secure cookie with CSRF token
		c.SetSameSite(http.SameSiteStrictMode)

		// Determine if cookie should be secure (HTTPS only)
		isSecure := h.config.IsHTTPS() || h.config.IsProduction()

		c.SetCookie(
			utils.CookieName,
			csrfToken,
			int(12*time.Hour.Seconds()), // 12 hour expiry
			"/",                         // Path
			"",                          // Domain (empty = current domain only)
			isSecure,                    // Secure (HTTPS only in production or when TLS enabled)
			true,                        // HTTP only
		)

		// Serve the React SPA index.html with CSRF token injected
		c.Header("Cache-Control", "no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		htmlBytes, err := os.ReadFile("./static/dist/index.html")
		if err != nil {
			logger.Logger.Error("Failed to read index.html", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load dashboard"})
			return
		}

		// Inject CSRF token as a meta tag
		html := strings.Replace(
			string(htmlBytes),
			"<head>",
			`<head><meta name="csrf-token" content="`+csrfToken+`">`,
			1,
		)

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	}
}
