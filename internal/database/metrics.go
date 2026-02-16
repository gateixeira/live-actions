package database

import (
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// InsertMetricsSnapshot records current running/queued job counts.
func (d *DBWrapper) InsertMetricsSnapshot(running, queued int) error {
	_, err := DB.Exec(
		"INSERT INTO metrics_snapshots (running_jobs, queued_jobs) VALUES (?, ?)",
		running, queued,
	)
	return err
}

// GetMetricsHistory returns time-series snapshots within the given duration.
func (d *DBWrapper) GetMetricsHistory(since time.Duration) ([]models.MetricsSnapshot, error) {
	cutoff := time.Now().UTC().Add(-since).Format("2006-01-02 15:04:05")
	rows, err := DB.Query(
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
func (d *DBWrapper) GetMetricsSummary(since time.Duration) (map[string]float64, error) {
	result := map[string]float64{
		"running_jobs":   0,
		"queued_jobs":    0,
		"avg_queue_time": 0,
		"peak_demand":    0,
	}

	// Current running and queued counts (live from workflow_jobs)
	row := DB.QueryRow(`SELECT
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
	err := DB.QueryRow(`SELECT COALESCE(AVG(
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
	err = DB.QueryRow(`SELECT COALESCE(MAX(running_jobs + queued_jobs), 0)
		FROM metrics_snapshots WHERE timestamp >= ?`, snapshotsCutoff).Scan(&peak)
	if err == nil {
		result["peak_demand"] = peak
	}

	return result, nil
}
