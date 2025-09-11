package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// InputValidator provides comprehensive input validation for common parameters
func InputValidator() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Validate common query parameters
		if err := validateQueryParams(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid query parameter: %s", err.Error()),
			})
			c.Abort()
			return
		}

		c.Next()
	})
}

// validateQueryParams validates common query parameters
func validateQueryParams(c *gin.Context) error {
	// Validate period parameter (if present)
	if period := c.Query("period"); period != "" {
		validPeriods := map[string]bool{
			"hour":   true,
			"day":    true,
			"week":   true,
			"month":  true,
			"custom": true,
		}
		if !validPeriods[period] {
			return fmt.Errorf("invalid period value: %s", period)
		}
	}

	// Validate time parameters (if present)
	timeParams := []string{"start_time", "end_time"}
	for _, param := range timeParams {
		if timeStr := c.Query(param); timeStr != "" {
			if err := validateTimeString(timeStr); err != nil {
				return fmt.Errorf("invalid %s: %s", param, err.Error())
			}
		}
	}

	return nil
}

// validateTimeString validates time string format
func validateTimeString(timeStr string) error {
	// Support common time formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if _, err := time.Parse(format, timeStr); err == nil {
			return nil
		}
	}

	return fmt.Errorf("unsupported time format")
}
