package handlers

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/utils"
	"github.com/gin-gonic/gin"
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

		// Compare hosts and path
		if refererURL.Host != requestHost || refererURL.Path != "/dashboard" {
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

		// Create template data with metrics
		templateData := gin.H{
			"csrfToken": csrfToken,
			"timestamp": time.Now().Unix(),
		}

		// Render template with data
		c.Header("Cache-Control", "no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.HTML(http.StatusOK, "dashboard.html", templateData)
	}
}
