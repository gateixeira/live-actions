package handlers

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/internal/utils"
	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type APIHandler struct {
	db     database.DatabaseInterface
	config *config.Config
}

func NewAPIHandler(config *config.Config, db database.DatabaseInterface) *APIHandler {
	return &APIHandler{
		db:     db,
		config: config,
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

		// Compare hostnames (ignore port to support dev proxy setups)
		refererHostname := refererURL.Hostname()
		requestHostname := requestHost
		if h, _, err := net.SplitHostPort(requestHost); err == nil {
			requestHostname = h
		}

		if refererHostname != requestHostname {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. This endpoint can only be accessed from the application.",
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

// ValidateSSEOrigin middleware validates the origin for SSE connections.
// EventSource does not support custom headers, so only the Referer/Origin
// header is checked (no CSRF token requirement).
//
// NOTE: Origin/Referer headers can be spoofed by non-browser clients.
// This middleware provides defense-in-depth against browser-based
// cross-origin attacks. For stronger protection, consider adding
// token-based authentication (e.g., signed query-string tokens).
func ValidateSSEOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try Origin header first (preferred), then fall back to Referer
		origin := c.Request.Header.Get("Origin")
		referer := c.Request.Header.Get("Referer")

		if origin == "" && referer == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. Missing origin.",
			})
			c.Abort()
			return
		}

		checkURL := origin
		if checkURL == "" {
			checkURL = referer
		}

		parsedURL, err := url.Parse(checkURL)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. Invalid origin.",
			})
			c.Abort()
			return
		}

		// Reject origins that don't contain a host
		originHost := parsedURL.Host
		if originHost == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. Origin must contain a host.",
			})
			c.Abort()
			return
		}

		// Compare full host:port to prevent cross-port attacks.
		// Normalize by adding default ports when missing.
		requestHost := c.Request.Host
		normalizedOriginHost := normalizeHost(parsedURL.Scheme, originHost)
		normalizedRequestHost := normalizeHost("", requestHost)

		if normalizedOriginHost != normalizedRequestHost {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. Cross-origin SSE connections are not allowed.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// normalizeHost ensures a host string has an explicit port by appending
// the default port for the given scheme when no port is present.
// For request hosts (scheme=""), it returns the host as-is.
func normalizeHost(scheme, host string) string {
	_, _, err := net.SplitHostPort(host)
	if err != nil {
		// No port present – add the default for the scheme
		switch scheme {
		case "https":
			return host + ":443"
		default:
			return host + ":80"
		}
	}
	return host
}

// GetWorkflowRuns retrieves the list of workflow runs from the database with pagination support
func (h *APIHandler) GetWorkflowRuns() gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit := GetPaginationParams(c)
		repo := c.Query("repo")
		status := c.Query("status")

		// Retrieve workflow runs from the database with pagination
		runs, totalCount, err := h.db.GetWorkflowRunsPaginated(c.Request.Context(), page, limit, repo, status)
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
		jobs, err := h.db.GetWorkflowJobsByRunID(c.Request.Context(), runIDInt64)
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

// GetCurrentMetrics returns current metrics and time-series data from the database.
func (h *APIHandler) GetCurrentMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		period := c.DefaultQuery("period", "day")

		since := periodToDuration(period)

		summary, err := h.db.GetMetricsSummary(c.Request.Context(), since)
		if err != nil {
			logger.Logger.Error("Failed to get metrics summary", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve metrics"})
			return
		}

		snapshots, err := h.db.GetMetricsHistory(c.Request.Context(), since)
		if err != nil {
			logger.Logger.Error("Failed to get metrics history", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve metrics"})
			return
		}

		// Build response in the same shape the frontend expects (Prometheus-compatible).
		runningValues := make([][]interface{}, len(snapshots))
		queuedValues := make([][]interface{}, len(snapshots))
		for i, s := range snapshots {
			runningValues[i] = []interface{}{s.Timestamp, fmt.Sprintf("%d", s.Running)}
			queuedValues[i] = []interface{}{s.Timestamp, fmt.Sprintf("%d", s.Queued)}
		}

		response := &models.MetricsResponse{
			CurrentMetrics: summary,
		}
		response.TimeSeries.RunningJobs = models.TimeSeriesData{
			Status: "success",
			Data: models.TimeSeriesDataInner{
				ResultType: "matrix",
				Result: []models.TimeSeriesEntry{{
					Metric: map[string]string{"job_status": "running"},
					Values: runningValues,
				}},
			},
		}
		response.TimeSeries.QueuedJobs = models.TimeSeriesData{
			Status: "success",
			Data: models.TimeSeriesDataInner{
				ResultType: "matrix",
				Result: []models.TimeSeriesEntry{{
					Metric: map[string]string{"job_status": "queued"},
					Values: queuedValues,
				}},
			},
		}

		c.JSON(http.StatusOK, response)
	}
}

func periodToDuration(period string) time.Duration {
	switch period {
	case "hour":
		return time.Hour
	case "week":
		return 7 * 24 * time.Hour
	case "month":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// GetFailureAnalytics returns failure summary and trend data for completed jobs.
func (h *APIHandler) GetFailureAnalytics() gin.HandlerFunc {
	return func(c *gin.Context) {
		period := c.DefaultQuery("period", "day")
		since := periodToDuration(period)
		ctx := c.Request.Context()
		repo := c.Query("repo")

		summary, err := h.db.GetFailureAnalytics(ctx, since, repo)
		if err != nil {
			logger.Logger.Error("Failed to get failure analytics", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve failure analytics"})
			return
		}

		trend, err := h.db.GetFailureTrend(ctx, since, repo)
		if err != nil {
			logger.Logger.Error("Failed to get failure trend", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve failure trend"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"summary": summary,
			"trend":   trend,
		})
	}
}

// GetLabelDemand returns per-label demand summary and trend data.
func (h *APIHandler) GetLabelDemand() gin.HandlerFunc {
	return func(c *gin.Context) {
		period := c.DefaultQuery("period", "day")
		since := periodToDuration(period)
		ctx := c.Request.Context()
		repo := c.Query("repo")

		summary, err := h.db.GetLabelDemandSummary(ctx, since, repo)
		if err != nil {
			logger.Logger.Error("Failed to get label demand summary", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve label demand"})
			return
		}

		trend, err := h.db.GetLabelDemandTrend(ctx, since, repo)
		if err != nil {
			logger.Logger.Error("Failed to get label demand trend", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve label demand trend"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"summary": summary,
			"trend":   trend,
		})
	}
}

// GetRepositories returns the list of distinct repository names.
func (h *APIHandler) GetRepositories() gin.HandlerFunc {
	return func(c *gin.Context) {
		repos, err := h.db.GetRepositories(c.Request.Context())
		if err != nil {
			logger.Logger.Error("Failed to get repositories", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve repositories"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"repositories": repos})
	}
}

// GetCSRFToken generates a CSRF token, sets it as a cookie, and returns it.
func (h *APIHandler) GetCSRFToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		csrfToken, err := utils.GenerateCSRFToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate security token"})
			return
		}

		c.SetSameSite(http.SameSiteStrictMode)
		isSecure := h.config.IsHTTPS() || h.config.IsProduction()

		c.SetCookie(
			utils.CookieName,
			csrfToken,
			int(12*time.Hour.Seconds()),
			"/",
			"",
			isSecure,
			true,
		)

		c.JSON(http.StatusOK, gin.H{"token": csrfToken})
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
