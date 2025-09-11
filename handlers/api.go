package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/internal/services"
	"github.com/gateixeira/live-actions/internal/utils"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type APIHandler struct {
	db                database.DatabaseInterface
	prometheusService services.PrometheusServiceInterface
}

func NewAPIHandler(config *config.Config, db database.DatabaseInterface, promSvc services.PrometheusServiceInterface) *APIHandler {
	return &APIHandler{
		db:                db,
		prometheusService: promSvc,
	}
}

// ValidateOrigin middleware ensures requests come from the UI
func ValidateOrigin() gin.HandlerFunc {
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

// GetWorkflowRuns retrieves the list of workflow runs from the database with pagination support
func (h *APIHandler) GetWorkflowRuns() gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit := GetPaginationParams(c)

		// Retrieve workflow runs from the database with pagination
		runs, totalCount, err := h.db.GetWorkflowRunsPaginated(page, limit)
		if err != nil {
			logger.Logger.Error("Error retrieving workflow runs", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve workflow runs"})
			return
		}

		// Calculate pagination metadata
		totalPages := (totalCount + limit - 1) / limit
		hasNext := page < totalPages
		hasPrev := page > 1

		// Return the workflow runs with pagination metadata as JSON
		c.JSON(http.StatusOK, gin.H{
			"workflow_runs": runs,
			"pagination": gin.H{
				"current_page": page,
				"total_pages":  totalPages,
				"total_count":  totalCount,
				"page_size":    limit,
				"has_next":     hasNext,
				"has_previous": hasPrev,
			},
		})
	}
}

func (h *APIHandler) GetWorkflowJobsByRunID() gin.HandlerFunc {
	return func(c *gin.Context) {
		runID := c.Param("run_id")

		// Convert runID to int64
		runIDInt64, err := strconv.ParseInt(runID, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run_id format"})
			return
		}

		// Retrieve workflow jobs for the given run ID from the database
		jobs, err := h.db.GetWorkflowJobsByRunID(runIDInt64)
		if err != nil {
			logger.Logger.Error("Error retrieving workflow jobs by run ID", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve workflow jobs"})
			return
		}

		if len(jobs) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "No workflow jobs found for this run ID"})
			return
		}

		// Return the workflow jobs as JSON
		c.JSON(http.StatusOK, gin.H{
			"workflow_jobs": jobs,
		})
	}
}

// GetLabelMetrics retrieves metrics broken down by label
func (h *APIHandler) GetLabelMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit := GetPaginationParams(c)

		jobs, totalCount, err := h.db.GetJobsByLabel(page, limit)
		if err != nil {
			logger.Logger.Error("Error retrieving jobs by label", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve jobs metrics"})
			return
		}

		// Calculate pagination metadata
		totalPages := (totalCount + limit - 1) / limit
		hasNext := page < totalPages
		hasPrev := page > 1

		c.JSON(http.StatusOK, gin.H{
			"label_metrics": jobs,
			"pagination": gin.H{
				"current_page": page,
				"total_pages":  totalPages,
				"total_count":  totalCount,
				"page_size":    limit,
				"has_next":     hasNext,
				"has_previous": hasPrev,
			},
		})
	}
}

// GetCurrentMetrics returns current calculated metrics using PromQL
func (h *APIHandler) GetCurrentMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this is a time series query (query_range parameters)
		start := c.Query("start")
		end := c.Query("end")
		step := c.Query("step")
		period := c.DefaultQuery("period", "day")

		results, err := h.prometheusService.GetMetricsWithTimeSeries(period, start, end, step)
		if err != nil {
			logger.Logger.Error("Failed to get metrics with time series", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve metrics"})
			return
		}
		c.JSON(http.StatusOK, results)
	}
}

func GetPaginationParams(c *gin.Context) (int, int) {
	// Parse pagination parameters
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "25")

	// Convert to integers with validation
	pageInt := 1
	limitInt := 25

	if p, err := fmt.Sscanf(page, "%d", &pageInt); err != nil || p != 1 || pageInt < 1 {
		pageInt = 1
	}

	if l, err := fmt.Sscanf(limit, "%d", &limitInt); err != nil || l != 1 || limitInt < 1 || limitInt > 100 {
		limitInt = 25
	}
	return pageInt, limitInt
}
