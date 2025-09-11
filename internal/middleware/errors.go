package middleware

import (
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ErrorHandler middleware for centralized error handling
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace (if logger is available)
				if logger.Logger != nil {
					logger.Logger.Error("Panic occurred",
						zap.Any("error", err),
						zap.String("path", c.Request.URL.Path),
						zap.String("method", c.Request.Method),
						zap.String("stack", string(debug.Stack())),
					)
				}

				// Return generic error response (don't expose internal details)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
				c.Abort()
			}
		}()

		c.Next()

		// Handle errors that were set during request processing
		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			// Log the error (if logger is available)
			if logger.Logger != nil {
				logger.Logger.Error("Request error",
					zap.Error(err.Err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.String("client_ip", c.ClientIP()),
				)
			}

			// Determine appropriate status code and response
			statusCode := http.StatusInternalServerError
			errorMessage := "Internal server error"

			// Check if it's a validation error or client error
			if c.Writer.Status() >= 400 && c.Writer.Status() < 500 {
				statusCode = c.Writer.Status()
				errorMessage = sanitizeErrorMessage(err.Error())
			}

			// Only send response if not already sent
			if !c.Writer.Written() {
				c.JSON(statusCode, gin.H{
					"error": errorMessage,
				})
			}
		}
	}
}

// RequestLogger logs incoming requests with security-relevant information
func RequestLogger() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			// Log security-relevant information (if logger is available)
			if logger.Logger != nil && param.Path != "/metrics" {
				logger.Logger.Debug("HTTP Request",
					zap.String("method", param.Method),
					zap.String("path", param.Path),
					zap.Int("status", param.StatusCode),
					zap.Duration("latency", param.Latency),
					zap.String("client_ip", param.ClientIP),
					zap.String("user_agent", param.Request.UserAgent()),
					zap.String("referer", param.Request.Referer()),
				)
			}
			return ""
		},
		Output: gin.DefaultWriter,
	})
}

// SecurityLogger logs security-relevant events
func SecurityLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Log failed authentication/authorization attempts
		if c.Writer.Status() == http.StatusUnauthorized || c.Writer.Status() == http.StatusForbidden {
			if logger.Logger != nil {
				logger.Logger.Warn("Access denied",
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.Int("status", c.Writer.Status()),
					zap.String("client_ip", c.ClientIP()),
					zap.String("user_agent", c.Request.UserAgent()),
				)
			}
		}
	}
}

// sanitizeErrorMessage removes sensitive information from error messages
func sanitizeErrorMessage(errMsg string) string {
	// List of sensitive patterns to remove/replace
	sensitivePatterns := map[string]string{
		"database": "storage",
		"sql":      "query",
		"postgres": "database",
		"password": "[REDACTED]",
		"secret":   "[REDACTED]",
		"key":      "[REDACTED]",
		"token":    "[REDACTED]",
	}

	sanitized := strings.ToLower(errMsg)
	for pattern, replacement := range sensitivePatterns {
		sanitized = strings.ReplaceAll(sanitized, pattern, replacement)
	}

	// Limit error message length
	if len(sanitized) > 100 {
		sanitized = sanitized[:97] + "..."
	}

	return sanitized
}

// getSanitizedHeaders returns headers with sensitive information removed
func getSanitizedHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string)

	// Only include security-relevant headers, exclude sensitive ones
	relevantHeaders := []string{
		"User-Agent", "Referer", "X-Forwarded-For", "X-Real-IP",
		"X-Forwarded-Proto", "Content-Type", "Accept",
	}

	for _, header := range relevantHeaders {
		if value := c.Request.Header.Get(header); value != "" {
			// Truncate very long headers
			if len(value) > 200 {
				value = value[:197] + "..."
			}
			headers[header] = value
		}
	}

	return headers
}
