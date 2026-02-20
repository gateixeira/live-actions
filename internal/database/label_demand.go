package database

import (
	"context"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// GetLabelDemandSummary returns per-label demand statistics for the given time window.
// If repo is non-empty, filters to that repository.
func (db *DBWrapper) GetLabelDemandSummary(ctx context.Context, since time.Duration, repo string) ([]models.LabelDemandSummary, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	repoJoin, repoArgs := jobRepoFilter(repo)
	args := append([]interface{}{cutoff}, repoArgs...)

	rows, err := db.db.QueryContext(ctx, `
		SELECT
			json_extract(j.labels, '$[0]') AS label,
			COUNT(*) AS total_jobs,
			SUM(CASE WHEN j.status = 'in_progress' THEN 1 ELSE 0 END) AS running,
			SUM(CASE WHEN j.status = 'queued' THEN 1 ELSE 0 END) AS queued,
			COALESCE(AVG(
				CASE WHEN j.started_at != '' AND j.created_at != ''
				THEN (julianday(j.started_at) - julianday(j.created_at)) * 86400
				END
			), 0) AS avg_queue_seconds
		FROM workflow_jobs j`+repoJoin+`
		WHERE j.created_at >= ? AND json_extract(j.labels, '$[0]') IS NOT NULL`+repoWhere(repo)+`
		GROUP BY label
		ORDER BY total_jobs DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get label demand summary: %w", err)
	}
	defer rows.Close()

	var results []models.LabelDemandSummary
	for rows.Next() {
		var s models.LabelDemandSummary
		if err := rows.Scan(&s.Label, &s.TotalJobs, &s.Running, &s.Queued, &s.AvgQueueSeconds); err != nil {
			return nil, fmt.Errorf("failed to scan label demand: %w", err)
		}
		results = append(results, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if results == nil {
		results = []models.LabelDemandSummary{}
	}

	return results, nil
}

// GetLabelDemandTrend returns time-bucketed per-label job counts.
// Uses hourly buckets for periods <= 1 day, daily buckets otherwise.
func (db *DBWrapper) GetLabelDemandTrend(ctx context.Context, since time.Duration, repo string) ([]models.LabelDemandTrendPoint, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	bucketFormat := "%Y-%m-%dT%H:00:00Z"
	if since > 24*time.Hour {
		bucketFormat = "%Y-%m-%dT00:00:00Z"
	}

	repoJoin, repoArgs := jobRepoFilter(repo)
	args := append([]interface{}{cutoff}, repoArgs...)

	rows, err := db.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			strftime('%s', j.created_at) AS bucket,
			json_extract(j.labels, '$[0]') AS label,
			COUNT(*) AS count
		FROM workflow_jobs j`+repoJoin+`
		WHERE j.created_at >= ? AND json_extract(j.labels, '$[0]') IS NOT NULL`+repoWhere(repo)+`
		GROUP BY bucket, label
		ORDER BY bucket ASC, label ASC`, bucketFormat), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get label demand trend: %w", err)
	}
	defer rows.Close()

	var points []models.LabelDemandTrendPoint
	for rows.Next() {
		var bucketStr string
		var p models.LabelDemandTrendPoint
		if err := rows.Scan(&bucketStr, &p.Label, &p.Count); err != nil {
			return nil, fmt.Errorf("failed to scan label demand trend: %w", err)
		}
		t, _ := time.Parse("2006-01-02T15:04:05Z", bucketStr)
		p.Timestamp = t.Unix()
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if points == nil {
		points = []models.LabelDemandTrendPoint{}
	}

	return points, nil
}
