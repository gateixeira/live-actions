package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
)

// labelsToJSON converts a string slice to a JSON string for storage
func labelsToJSON(labels []string) string {
	if labels == nil {
		labels = []string{}
	}
	b, _ := json.Marshal(labels)
	return string(b)
}

// labelsFromJSON converts a JSON string back to a string slice
func labelsFromJSON(s string) []string {
	var labels []string
	if s == "" {
		return []string{}
	}
	json.Unmarshal([]byte(s), &labels)
	if labels == nil {
		return []string{}
	}
	return labels
}

// AddOrUpdateJob adds or updates a workflow job with atomicity checks.
// It prevents older events from overwriting newer terminal states.
// Returns (updated, error) where updated indicates if the job was actually updated.
func (db *DBWrapper) AddOrUpdateJob(workflowJob models.WorkflowJob, eventTimestamp time.Time) (bool, error) {
	tx, err := DB.Begin()
	if err != nil {
		time.Sleep(time.Millisecond * 100)
		return false, fmt.Errorf("failed to start transaction: %w", err)
	}

	var isTerminal bool
	err = tx.QueryRow(`
		SELECT CASE WHEN status IN ('completed', 'cancelled') THEN 1 ELSE 0 END
		FROM workflow_jobs 
		WHERE id = ?`, workflowJob.ID).Scan(&isTerminal)

	if err != nil && err != sql.ErrNoRows {
		tx.Rollback()
		return false, fmt.Errorf("failed to check terminal state: %w", err)
	}

	if err == nil && isTerminal {
		tx.Rollback()
		return false, nil
	}

	_, err = tx.Exec(
		`INSERT INTO workflow_jobs (id, name, status, labels, html_url, conclusion, created_at, started_at, completed_at, updated_at, run_id) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), ?)
		ON CONFLICT (id) DO UPDATE SET
			name = excluded.name,
			status = excluded.status,
			labels = excluded.labels,
			html_url = excluded.html_url,
			conclusion = excluded.conclusion,
			created_at = excluded.created_at,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			updated_at = datetime('now'),
			run_id = excluded.run_id`,
		workflowJob.ID, string(workflowJob.Name), string(workflowJob.Status), labelsToJSON(workflowJob.Labels),
		workflowJob.HtmlUrl, string(workflowJob.Conclusion), workflowJob.CreatedAt.Format(time.RFC3339), formatNullableTime(workflowJob.StartedAt), formatNullableTime(workflowJob.CompletedAt), workflowJob.RunID,
	)

	if err != nil {
		tx.Rollback()
		return false, fmt.Errorf("failed to execute upsert: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return true, nil
}

func (db *DBWrapper) AddOrUpdateRun(workflowRun models.WorkflowRun, eventTimestamp time.Time) (bool, error) {
	tx, err := DB.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to start transaction: %w", err)
	}

	var isTerminal bool
	err = tx.QueryRow(`
		SELECT CASE WHEN status IN ('completed', 'cancelled') THEN 1 ELSE 0 END
		FROM workflow_runs 
		WHERE id = ?`, workflowRun.ID).Scan(&isTerminal)

	if err != nil && err != sql.ErrNoRows {
		tx.Rollback()
		return false, fmt.Errorf("failed to check terminal state: %w", err)
	}

	if err == nil && isTerminal {
		tx.Rollback()
		return false, nil
	}

	_, err = tx.Exec(
		`INSERT INTO workflow_runs (id, name, status, repository,
		html_url, display_title, conclusion, created_at, run_started_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = excluded.name,
			status = excluded.status,
			repository = excluded.repository,
			html_url = excluded.html_url,
			display_title = excluded.display_title,
			conclusion = excluded.conclusion,
			created_at = excluded.created_at,
			run_started_at = excluded.run_started_at,
			updated_at = excluded.updated_at`,
		workflowRun.ID, string(workflowRun.Name), string(workflowRun.Status), string(workflowRun.RepositoryName),
		string(workflowRun.HtmlUrl), string(workflowRun.DisplayTitle), string(workflowRun.Conclusion),
		workflowRun.CreatedAt.Format(time.RFC3339), formatNullableTime(workflowRun.RunStartedAt), formatNullableTime(workflowRun.UpdatedAt),
	)

	if err != nil {
		tx.Rollback()
		return false, fmt.Errorf("failed to execute upsert: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return true, nil
}

// GetWorkflowRunsPaginated retrieves workflow runs with pagination support
func (db *DBWrapper) GetWorkflowRunsPaginated(page int, limit int) ([]models.WorkflowRun, int, error) {
	offset := (page - 1) * limit

	var totalCount int
	err := DB.QueryRow("SELECT COUNT(*) FROM workflow_runs").Scan(&totalCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return []models.WorkflowRun{}, 0, nil
		}
		return nil, 0, err
	}

	rows, err := DB.Query("SELECT id, name, status, repository, html_url, display_title, conclusion, created_at, run_started_at, updated_at FROM workflow_runs ORDER BY created_at DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var runs []models.WorkflowRun
	for rows.Next() {
		var run models.WorkflowRun
		var createdAt, startedAt, updatedAt sql.NullString
		if err := rows.Scan(&run.ID, &run.Name, &run.Status, &run.RepositoryName, &run.HtmlUrl, &run.DisplayTitle, &run.Conclusion, &createdAt, &startedAt, &updatedAt); err != nil {
			return nil, 0, err
		}
		run.CreatedAt = parseTime(createdAt.String)
		run.RunStartedAt = parseTime(startedAt.String)
		run.UpdatedAt = parseTime(updatedAt.String)
		runs = append(runs, run)
	}

	return runs, totalCount, nil
}

func (db *DBWrapper) GetWorkflowJobsByRunID(runID int64) ([]models.WorkflowJob, error) {
	rows, err := DB.Query("SELECT id, name, run_id, status, labels, html_url, conclusion, created_at, started_at, completed_at FROM workflow_jobs WHERE run_id = ? ORDER BY created_at DESC", runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.WorkflowJob
	for rows.Next() {
		var job models.WorkflowJob
		var labelsJSON string
		var createdAt string
		var htmlUrl sql.NullString
		var startedAt, completedAt sql.NullString
		if err := rows.Scan(&job.ID, &job.Name, &job.RunID, &job.Status, &labelsJSON, &htmlUrl, &job.Conclusion, &createdAt, &startedAt, &completedAt); err != nil {
			return nil, err
		}
		job.Labels = labelsFromJSON(labelsJSON)
		job.HtmlUrl = htmlUrl.String
		job.CreatedAt = parseTime(createdAt)
		job.StartedAt = parseTime(startedAt.String)
		job.CompletedAt = parseTime(completedAt.String)
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetWorkflowJobByID retrieves a single workflow job by its ID
func (db *DBWrapper) GetWorkflowJobByID(jobID int64) (models.WorkflowJob, error) {
	var job models.WorkflowJob
	var labelsJSON string
	var createdAt string
	var htmlUrl sql.NullString
	var startedAt, completedAt sql.NullString

	err := DB.QueryRow(`
		SELECT id, name, run_id, status, labels, html_url, conclusion, 
			   created_at, started_at, completed_at 
		FROM workflow_jobs 
		WHERE id = ?`, jobID).Scan(
		&job.ID, &job.Name, &job.RunID, &job.Status,
		&labelsJSON, &htmlUrl, &job.Conclusion, &createdAt,
		&startedAt, &completedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return models.WorkflowJob{Status: ""}, nil
		}
		return models.WorkflowJob{}, err
	}

	job.Labels = labelsFromJSON(labelsJSON)
	job.HtmlUrl = htmlUrl.String
	job.CreatedAt = parseTime(createdAt)
	job.StartedAt = parseTime(startedAt.String)
	job.CompletedAt = parseTime(completedAt.String)

	return job, nil
}

// CleanupOldData removes workflow runs and jobs older than the retention period
func (db *DBWrapper) CleanupOldData(retentionPeriod time.Duration) (int64, int64, int64, error) {
	cutoffTime := time.Now().Add(-retentionPeriod).Format(time.RFC3339)

	tx, err := DB.Begin()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	jobResult, err := tx.Exec("DELETE FROM workflow_jobs WHERE created_at < ?", cutoffTime)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to delete old workflow jobs: %w", err)
	}

	deletedJobs, err := jobResult.RowsAffected()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get affected jobs count: %w", err)
	}

	runResult, err := tx.Exec("DELETE FROM workflow_runs WHERE created_at < ?", cutoffTime)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to delete old workflow runs: %w", err)
	}

	deletedRuns, err := runResult.RowsAffected()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get affected runs count: %w", err)
	}

	eventResult, err := tx.Exec("DELETE FROM webhook_events WHERE processed_at < ?", cutoffTime)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to delete old webhook events: %w", err)
	}

	deletedEvents, err := eventResult.RowsAffected()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get affected events count: %w", err)
	}

	// Clean up old metrics snapshots
	if _, err := tx.Exec("DELETE FROM metrics_snapshots WHERE timestamp < ?", cutoffTime); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to delete old metrics snapshots: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to commit cleanup transaction: %w", err)
	}

	return deletedRuns, deletedJobs, deletedEvents, nil
}

func (db *DBWrapper) GetCurrentJobCounts() (int, int, error) {
	var running, queued int
	err := DB.QueryRow(`
		SELECT 
			COALESCE(SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END), 0)
		FROM workflow_jobs
	`).Scan(&running, &queued)
	if err != nil {
		return 0, 0, err
	}
	return running, queued, nil
}

// formatNullableTime formats a time.Time as RFC3339 string, returning nil for zero times
func formatNullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}

// parseTime parses an RFC3339 string into time.Time, returning zero time on failure
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try other formats
		t, err = time.Parse("2006-01-02 15:04:05-07:00", s)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05Z", s)
			if err != nil {
				return time.Time{}
			}
		}
	}
	return t
}
