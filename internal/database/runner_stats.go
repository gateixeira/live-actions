package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// GetRunnerStats returns per-runner job statistics, sorted by runner name.
// Job counts, failure rate, average duration and last job time cover jobs
// completed within the given time window; Running counts jobs currently in
// progress on each runner regardless of the window. Jobs without a runner
// (never picked up) are skipped. If repo is non-empty, filters to that
// repository.
func (db *DBWrapper) GetRunnerStats(ctx context.Context, since time.Duration, repo string) ([]models.RunnerStats, error) {
	// completed_at is stored as UTC RFC3339, so the cutoff must be UTC too
	// for the string comparison to hold.
	cutoff := time.Now().UTC().Add(-since).Format(time.RFC3339)

	repoJoin, repoArgs := jobRepoFilter(repo)
	args := append([]interface{}{cutoff}, repoArgs...)

	// The WHERE clause keeps only jobs completed in the window plus jobs in
	// progress now, so status = 'completed' alone identifies in-window jobs.
	// A runner belongs to one group; MAX picks the non-empty name if any.
	rows, err := db.readDB.QueryContext(ctx, `
		SELECT
			j.runner_name,
			COALESCE(MAX(j.runner_group_name), '') AS runner_group_name,
			SUM(CASE WHEN j.status = 'completed' THEN 1 ELSE 0 END) AS total_jobs,
			SUM(CASE WHEN j.status = 'completed' AND j.conclusion = 'success' THEN 1 ELSE 0 END) AS succeeded,
			SUM(CASE WHEN j.status = 'completed' AND j.conclusion IN ('failure','timed_out') THEN 1 ELSE 0 END) AS failed,
			SUM(CASE WHEN j.status = 'completed' AND j.conclusion = 'cancelled' THEN 1 ELSE 0 END) AS cancelled,
			COALESCE(AVG(
				CASE WHEN j.status = 'completed' AND j.started_at != ''
				THEN (julianday(j.completed_at) - julianday(j.started_at)) * 86400
				END
			), 0) AS avg_duration_seconds,
			SUM(CASE WHEN j.status = 'in_progress' THEN 1 ELSE 0 END) AS running,
			MAX(CASE WHEN j.status = 'completed' THEN j.completed_at END) AS last_job_at
		FROM workflow_jobs j`+repoJoin+`
		WHERE j.runner_name != ''
			AND ((j.status = 'completed' AND j.completed_at >= ?) OR j.status = 'in_progress')`+repoWhere(repo)+`
		GROUP BY j.runner_name
		ORDER BY j.runner_name ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get runner stats: %w", err)
	}
	defer rows.Close()

	var results []models.RunnerStats
	for rows.Next() {
		var s models.RunnerStats
		var lastJobAt sql.NullString
		if err := rows.Scan(&s.RunnerName, &s.RunnerGroupName, &s.TotalJobs, &s.Succeeded, &s.Failed,
			&s.Cancelled, &s.AvgDurationSeconds, &s.Running, &lastJobAt); err != nil {
			return nil, fmt.Errorf("failed to scan runner stats: %w", err)
		}
		s.Other = s.TotalJobs - s.Succeeded - s.Failed - s.Cancelled
		if s.TotalJobs > 0 {
			s.FailureRate = float64(s.Failed) / float64(s.TotalJobs)
		}
		if lastJobAt.Valid {
			if t := parseTime(lastJobAt.String); !t.IsZero() {
				s.LastJobAt = &t
			}
		}
		results = append(results, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if results == nil {
		results = []models.RunnerStats{}
	}

	return results, nil
}
