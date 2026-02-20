package database

import (
	"context"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// GetFailureAnalytics returns failure summary statistics for completed jobs
// within the given time window. If repo is non-empty, filters to that repository.
func (db *DBWrapper) GetFailureAnalytics(ctx context.Context, since time.Duration, repo string) (*models.FailureAnalytics, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	repoJoin, repoArgs := jobRepoFilter(repo)

	var totalCompleted, totalFailed, totalCancelled int
	args := append([]interface{}{cutoff}, repoArgs...)
	err := db.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN j.conclusion IN ('failure','timed_out') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN j.conclusion = 'cancelled' THEN 1 ELSE 0 END), 0)
		FROM workflow_jobs j`+repoJoin+`
		WHERE j.status = 'completed' AND j.completed_at >= ?`+repoWhere(repo), args...).Scan(&totalCompleted, &totalFailed, &totalCancelled)
	if err != nil {
		return nil, fmt.Errorf("failed to get failure summary: %w", err)
	}

	var failureRate float64
	if totalCompleted > 0 {
		failureRate = float64(totalFailed) / float64(totalCompleted) * 100
	}

	rows, err := db.db.QueryContext(ctx, `
		SELECT
			j.name,
			MAX(j.html_url) AS html_url,
			SUM(CASE WHEN j.conclusion IN ('failure','timed_out') THEN 1 ELSE 0 END) AS failures,
			COUNT(*) AS total
		FROM workflow_jobs j`+repoJoin+`
		WHERE j.status = 'completed' AND j.completed_at >= ?`+repoWhere(repo)+`
		GROUP BY j.name
		HAVING failures > 0
		ORDER BY failures DESC
		LIMIT 10`, args...)
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
func (db *DBWrapper) GetFailureTrend(ctx context.Context, since time.Duration, repo string) ([]models.FailureTrendPoint, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	bucketFormat := "%Y-%m-%dT%H:00:00Z"
	if since > 24*time.Hour {
		bucketFormat = "%Y-%m-%dT00:00:00Z"
	}

	repoJoin, repoArgs := jobRepoFilter(repo)
	args := append([]interface{}{cutoff}, repoArgs...)

	rows, err := db.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			strftime('%s', j.completed_at) AS bucket,
			COALESCE(SUM(CASE WHEN j.conclusion IN ('failure','timed_out') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN j.conclusion = 'success' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN j.conclusion = 'cancelled' THEN 1 ELSE 0 END), 0)
		FROM workflow_jobs j`+repoJoin+`
		WHERE j.status = 'completed' AND j.completed_at >= ?`+repoWhere(repo)+`
		GROUP BY bucket
		ORDER BY bucket ASC`, bucketFormat), args...)
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
