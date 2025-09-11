package services

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"go.uber.org/zap"
)

type PrometheusServiceInterface interface {
	GetMetricsWithTimeSeries(period, start, end, step string) (*models.MetricsResponse, error)
	QueryPrometheus(path string, queryParams url.Values) ([]byte, error)
}

// PrometheusService handles Prometheus queries
type PrometheusService struct {
	prometheusURL string
	client        *http.Client
}

// sanitizeFloat64 converts NaN and Inf to 0 for JSON marshaling
func sanitizeFloat64(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

// NewPrometheusService creates a new Prometheus service
func NewPrometheusService(url string) PrometheusServiceInterface {
	logger.Logger.Info("Initializing Prometheus service", zap.String("url", url))
	return &PrometheusService{
		prometheusURL: url,
		client:        &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *PrometheusService) QueryPrometheus(path string, queryParams url.Values) ([]byte, error) {
	if p.prometheusURL == "" {
		return nil, fmt.Errorf("PROMETHEUS_URL environment variable not set")
	}

	proxyURL, err := url.Parse(p.prometheusURL + "/api/v1/" + path)
	if err != nil {
		logger.Logger.Error("Invalid Prometheus URL", zap.Error(err))
		return nil, fmt.Errorf("invalid Prometheus URL: %w", err)
	}

	proxyURL.RawQuery = queryParams.Encode()

	// Add detailed logging
	logger.Logger.Debug("Making Prometheus request",
		zap.String("url", proxyURL.String()),
		zap.String("query", queryParams.Get("query")))

	req, err := http.NewRequest("GET", proxyURL.String(), nil)
	if err != nil {
		logger.Logger.Error("Failed to create Prometheus request", zap.Error(err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		logger.Logger.Error("Failed to query Prometheus",
			zap.Error(err),
			zap.String("url", proxyURL.String()))
		return nil, fmt.Errorf("failed to query Prometheus: %w", err)
	}
	defer resp.Body.Close()

	// Log response status
	logger.Logger.Debug("Prometheus response",
		zap.Int("status_code", resp.StatusCode),
		zap.String("status", resp.Status))

	if resp.StatusCode != http.StatusOK {
		logger.Logger.Error("Prometheus returned non-200 status",
			zap.Int("status_code", resp.StatusCode),
			zap.String("status", resp.Status))
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.Error("Failed to read Prometheus response", zap.Error(err))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	logger.Logger.Debug("Prometheus response body",
		zap.String("body", string(body))) // Log first 500 chars

	return body, nil
}

// QueryRange executes a range query against Prometheus and returns the response body
func (p *PrometheusService) QueryRange(queryParams url.Values) ([]byte, error) {
	return p.QueryPrometheus("query_range", queryParams)
}

// QueryValue executes a PromQL query and returns a single scalar value
func (p *PrometheusService) QueryValue(query string) (float64, error) {
	params := url.Values{}
	params.Set("query", query)

	result, err := p.QueryPrometheus("query", params)

	if err != nil {
		return 0, fmt.Errorf("failed to query Prometheus: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(result, &response); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if data, ok := response["data"].(map[string]interface{}); ok {
		if resultArray, ok := data["result"].([]interface{}); ok && len(resultArray) > 0 {
			if firstResult, ok := resultArray[0].(map[string]interface{}); ok {
				if value, ok := firstResult["value"].([]interface{}); ok && len(value) >= 2 {
					if valueStr, ok := value[1].(string); ok {
						return strconv.ParseFloat(valueStr, 64)
					}
				}
			}
		}
	}

	return 0, nil // No data found
}

// GetMetricsWithTimeSeries returns both current metrics and time series data for the given period
func (p *PrometheusService) GetMetricsWithTimeSeries(period, start, end, step string) (*models.MetricsResponse, error) {
	// Get current metrics with period-adjusted queries
	currentMetrics, err := p.GetPeriodSpecificMetrics(period)
	if err != nil {
		return nil, fmt.Errorf("failed to get current metrics: %w", err)
	}

	params := url.Values{}
	params.Set("start", start)
	params.Set("end", end)
	params.Set("step", step)

	runningJobsQuery := `github_runners_jobs{job_status="running"}`
	queuedJobsQuery := `github_runners_jobs{job_status="queued"}`

	// Query running jobs time series
	params.Set("query", runningJobsQuery)
	runningResult, err := p.QueryRange(params)
	if err != nil {
		logger.Logger.Error("Failed to query running jobs time series", zap.Error(err))
		runningResult = []byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`)
	}

	// Query queued jobs time series
	params.Set("query", queuedJobsQuery)
	queuedResult, err := p.QueryRange(params)
	if err != nil {
		logger.Logger.Error("Failed to query queued jobs time series", zap.Error(err))
		queuedResult = []byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`)
	}

	// Parse results directly without combining
	var runningData, queuedData models.TimeSeriesData
	if err := json.Unmarshal(runningResult, &runningData); err != nil {
		logger.Logger.Error("Failed to unmarshal running jobs data", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal running jobs data: %w", err)
	}

	if err := json.Unmarshal(queuedResult, &queuedData); err != nil {
		logger.Logger.Error("Failed to unmarshal queued jobs data", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal queued jobs data: %w", err)
	}

	response := &models.MetricsResponse{
		CurrentMetrics: currentMetrics,
	}
	response.TimeSeries.RunningJobs = runningData
	response.TimeSeries.QueuedJobs = queuedData

	return response, nil
}

// GetPeriodSpecificMetrics returns metrics with period-adjusted time ranges
func (p *PrometheusService) GetPeriodSpecificMetrics(period string) (map[string]float64, error) {

	var queueTimeRange, peakTimeRange string

	switch period {
	case "hour":
		queueTimeRange = "1h"
		peakTimeRange = "1h"
	case "week":
		queueTimeRange = "7d"
		peakTimeRange = "7d"
	case "month":
		queueTimeRange = "30d"
		peakTimeRange = "30d"
	default: // day
		queueTimeRange = "24h"
		peakTimeRange = "24h"
	}

	queries := map[string]string{
		"running_jobs": `sum(github_runners_jobs{job_status="running"}) or vector(0)`,
		"queued_jobs":  `sum(github_runners_jobs{job_status="queued"}) or vector(0)`,

		"avg_queue_time": fmt.Sprintf(`avg(rate(github_runners_queue_duration_seconds_sum[%s]) / 
                           rate(github_runners_queue_duration_seconds_count[%s])) or vector(0)`,
			queueTimeRange, queueTimeRange),

		"peak_demand": fmt.Sprintf(`max_over_time((sum(github_runners_jobs{job_status=~"running|queued"}) or vector(0))[%s:1m])`,
			peakTimeRange),
	}

	results := make(map[string]float64)
	for name, query := range queries {
		value, err := p.QueryValue(query)
		if err != nil {
			logger.Logger.Error("Failed to query Prometheus",
				zap.String("metric", name),
				zap.String("period", period),
				zap.Error(err))

			results[name] = 0
		} else {
			results[name] = sanitizeFloat64(value)
		}
	}

	return results, nil
}
