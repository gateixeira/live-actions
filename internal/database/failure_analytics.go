package database

import (
	"context"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// failureConclusions lists conclusions that count as failures for workflow jobs.
// Note: "startup_failure" is only valid for workflow_runs, not workflow_jobs.
var failureConclusions = []string{"failure", "timed_out"}

// GetFailureAnalytics returns failure summary statistics for completed jobs
// within the given time window.
func (db *DBWrapper) GetFailureAnalytics(ctx context.Context, since time.Duration) (*models.FailureAnalytics, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	var totalCompleted, totalFailed, totalCancelled int
	err := db.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN conclusion IN ('failure','timed_out') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN conclusion = 'cancelled' THEN 1 ELSE 0 END), 0)
		FROM workflow_jobs
		WHERE status = 'completed' AND completed_at >= ?`, cutoff).Scan(&totalCompleted, &totalFailed, &totalCancelled)
	if err != nil {
		return nil, fmt.Errorf("failed to get failure summary: %w", err)
	}

	var failureRate float64
	if totalCompleted > 0 {
		failureRate = float64(totalFailed) / float64(totalCompleted) * 100
	}

	rows, err := db.db.QueryContext(ctx, `
		SELECT
			name,
			MAX(html_url) AS html_url,
			SUM(CASE WHEN conclusion IN ('failure','timed_out') THEN 1 ELSE 0 END) AS failures,
			COUNT(*) AS total
		FROM workflow_jobs
		WHERE status = 'completed' AND completed_at >= ?
		GROUP BY name
		HAVING failures > 0
		ORDER BY failures DESC
		LIMIT 10`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to get top failing jobs: %w", err)
	}
	defer rows.Close()

	var topFailing []models.FailingJob
	for rows.Next() {
		var j models.FailingJob
		if err := rows.Scan(&j.Name, &j.HtmlUrl, &j.Failures, &j.Total); err != nil {
			return nil, fmt.Errorf("failed to scan failing job: %w", err)
		}
		if j.Total > 0 {
			j.FailureRate = float64(j.Failures) / float64(j.Total) * 100
		}
		topFailing = append(topFailing, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if topFailing == nil {
		topFailing = []models.FailingJob{}
	}

	return &models.FailureAnalytics{
		TotalCompleted: totalCompleted,
		TotalFailed:    totalFailed,
		TotalCancelled: totalCancelled,
		FailureRate:    failureRate,
		TopFailingJobs: topFailing,
	}, nil
}

// GetFailureTrend returns time-bucketed failure/success/cancelled counts.
// Uses hourly buckets for periods <= 1 day, daily buckets otherwise.
func (db *DBWrapper) GetFailureTrend(ctx context.Context, since time.Duration) ([]models.FailureTrendPoint, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	// Choose bucket format: hourly for <= 1 day, daily otherwise
	bucketFormat := "%Y-%m-%dT%H:00:00Z"
	if since > 24*time.Hour {
		bucketFormat = "%Y-%m-%dT00:00:00Z"
	}

	rows, err := db.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			strftime('%s', completed_at) AS bucket,
			COALESCE(SUM(CASE WHEN conclusion IN ('failure','timed_out') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN conclusion = 'success' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN conclusion = 'cancelled' THEN 1 ELSE 0 END), 0)
		FROM workflow_jobs
		WHERE status = 'completed' AND completed_at >= ?
		GROUP BY bucket
		ORDER BY bucket ASC`, bucketFormat), cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to get failure trend: %w", err)
	}
	defer rows.Close()

	var points []models.FailureTrendPoint
	for rows.Next() {
		var bucketStr string
		var p models.FailureTrendPoint
		if err := rows.Scan(&bucketStr, &p.Failures, &p.Successes, &p.Cancelled); err != nil {
			return nil, fmt.Errorf("failed to scan trend point: %w", err)
		}
		t, _ := time.Parse("2006-01-02T15:04:05Z", bucketStr)
		p.Timestamp = t.Unix()
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if points == nil {
		points = []models.FailureTrendPoint{}
	}

	return points, nil
}
