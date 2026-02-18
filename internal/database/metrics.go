package database

import (
	"context"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// InsertMetricsSnapshot records current running/queued job counts.
func (d *DBWrapper) InsertMetricsSnapshot(ctx context.Context, running, queued int) error {
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO metrics_snapshots (running_jobs, queued_jobs) VALUES (?, ?)",
		running, queued,
	)
	return err
}

// GetMetricsHistory returns time-series snapshots within the given duration.
func (d *DBWrapper) GetMetricsHistory(ctx context.Context, since time.Duration) ([]models.MetricsSnapshot, error) {
	cutoff := time.Now().UTC().Add(-since).Format("2006-01-02 15:04:05")
	rows, err := d.db.QueryContext(ctx,
		`SELECT timestamp, running_jobs, queued_jobs
		 FROM metrics_snapshots
		 WHERE timestamp >= ?
		 ORDER BY timestamp ASC`, cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics history: %w", err)
	}
	defer rows.Close()

	var snapshots []models.MetricsSnapshot
	for rows.Next() {
		var s models.MetricsSnapshot
		var ts string
		if err := rows.Scan(&ts, &s.Running, &s.Queued); err != nil {
			return nil, fmt.Errorf("failed to scan metrics snapshot: %w", err)
		}
		t, _ := time.Parse("2006-01-02 15:04:05", ts)
		if t.IsZero() {
			t, _ = time.Parse(time.RFC3339, ts)
		}
		s.Timestamp = t.Unix()
		snapshots = append(snapshots, s)
	}
	return snapshots, rows.Err()
}

// GetMetricsSummary computes running_jobs, queued_jobs, avg_queue_time, and peak_demand
// from the database for the given time window.
func (d *DBWrapper) GetMetricsSummary(ctx context.Context, since time.Duration) (map[string]float64, error) {
	result := map[string]float64{
		"running_jobs":   0,
		"queued_jobs":    0,
		"avg_queue_time": 0,
		"peak_demand":    0,
	}

	// Current running and queued counts (live from workflow_jobs)
	row := d.db.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END), 0)
		FROM workflow_jobs`)
	var running, queued float64
	if err := row.Scan(&running, &queued); err != nil {
		return result, fmt.Errorf("failed to get current job counts: %w", err)
	}
	result["running_jobs"] = running
	result["queued_jobs"] = queued

	// workflow_jobs stores timestamps as RFC3339
	jobsCutoff := time.Now().UTC().Add(-since).Format(time.RFC3339)

	// Average queue time: average seconds between created_at and started_at for
	// jobs that started within the period.
	var avgQueue float64
	err := d.db.QueryRowContext(ctx, `SELECT COALESCE(AVG(
		(julianday(started_at) - julianday(created_at)) * 86400
	), 0) FROM workflow_jobs
	WHERE started_at IS NOT NULL AND started_at >= ?`, jobsCutoff).Scan(&avgQueue)
	if err == nil {
		result["avg_queue_time"] = avgQueue
	}

	// metrics_snapshots stores timestamps as datetime (no T, no Z)
	snapshotsCutoff := time.Now().UTC().Add(-since).Format("2006-01-02 15:04:05")

	// Peak demand from snapshots (max of running + queued in the period)
	var peak float64
	err = d.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(running_jobs + queued_jobs), 0)
		FROM metrics_snapshots WHERE timestamp >= ?`, snapshotsCutoff).Scan(&peak)
	if err == nil {
		result["peak_demand"] = peak
	}

	return result, nil
}

// LabelJobCount holds running/queued counts for a single runner label.
type LabelJobCount struct {
	Label   string
	Running int
	Queued  int
}

// GetCurrentJobCountsByLabel returns current running and queued counts grouped by the first label.
func (d *DBWrapper) GetCurrentJobCountsByLabel(ctx context.Context) ([]LabelJobCount, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT
			COALESCE(json_extract(labels, '$[0]'), '(unlabeled)') AS label,
			SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END) AS running,
			SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END) AS queued
		FROM workflow_jobs
		WHERE status IN ('in_progress', 'queued')
		GROUP BY label`)
	if err != nil {
		return nil, fmt.Errorf("failed to get job counts by label: %w", err)
	}
	defer rows.Close()

	var counts []LabelJobCount
	for rows.Next() {
		var c LabelJobCount
		if err := rows.Scan(&c.Label, &c.Running, &c.Queued); err != nil {
			return nil, fmt.Errorf("failed to scan label job count: %w", err)
		}
		counts = append(counts, c)
	}
	return counts, rows.Err()
}
