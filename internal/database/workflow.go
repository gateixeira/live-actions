package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/lib/pq"
)

// AddOrUpdateJob adds or updates a workflow job with atomicity checks.
// It prevents older events from overwriting newer terminal states.
// Returns (updated, error) where updated indicates if the job was actually updated.
func (db *DBWrapper) AddOrUpdateJob(workflowJob models.WorkflowJob, eventTimestamp time.Time) (bool, error) {
	// Start transaction
	tx, err := DB.Begin()
	if err != nil {
		time.Sleep(time.Millisecond * 100)
		return false, fmt.Errorf("failed to start transaction: %w", err)
	}

	var isTerminal bool
	err = tx.QueryRow(`
		SELECT CASE WHEN status IN ('completed', 'cancelled') THEN true ELSE false END as is_terminal
		FROM workflow_jobs 
		WHERE id = $1 FOR UPDATE`, workflowJob.ID).Scan(&isTerminal)

	if err != nil && err != sql.ErrNoRows && isTerminal {
		tx.Rollback()
		return false, nil
	}

	_, err = tx.Exec(
		`INSERT INTO workflow_jobs (id, name, status, runner_type, labels, conclusion, created_at, started_at, completed_at, updated_at, run_id) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), $10)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			runner_type = EXCLUDED.runner_type,
			labels = EXCLUDED.labels,
			conclusion = EXCLUDED.conclusion,
			created_at = EXCLUDED.created_at,
			started_at = EXCLUDED.started_at,
			completed_at = EXCLUDED.completed_at,
			updated_at = NOW(),
			run_id = EXCLUDED.run_id`,
		workflowJob.ID, string(workflowJob.Name), string(workflowJob.Status), string(workflowJob.RunnerType), pq.Array(workflowJob.Labels),
		string(workflowJob.Conclusion), workflowJob.CreatedAt, workflowJob.StartedAt, workflowJob.CompletedAt, workflowJob.RunID,
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
	// Start transaction
	tx, err := DB.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to start transaction: %w", err)
	}

	var isTerminal bool
	err = tx.QueryRow(`
		SELECT CASE WHEN status IN ('completed', 'cancelled') THEN true ELSE false END as is_terminal
		FROM workflow_runs 
		WHERE id = $1 FOR UPDATE`, workflowRun.ID).Scan(&isTerminal)

	if err != nil && err != sql.ErrNoRows && isTerminal {
		tx.Rollback()
		return false, nil
	}

	_, err = tx.Exec(
		`INSERT INTO workflow_runs (id, name, status, repository,
		html_url, display_title, conclusion, created_at, run_started_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			repository = EXCLUDED.repository,
			html_url = EXCLUDED.html_url,
			display_title = EXCLUDED.display_title,
			conclusion = EXCLUDED.conclusion,
			created_at = EXCLUDED.created_at,
			run_started_at = EXCLUDED.run_started_at,
			updated_at = EXCLUDED.updated_at`,
		workflowRun.ID, string(workflowRun.Name), string(workflowRun.Status), string(workflowRun.RepositoryName),
		string(workflowRun.HtmlUrl), string(workflowRun.DisplayTitle), string(workflowRun.Conclusion),
		workflowRun.CreatedAt, workflowRun.RunStartedAt, workflowRun.UpdatedAt,
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
	// Calculate offset
	offset := (page - 1) * limit

	// Get total count
	var totalCount int
	err := DB.QueryRow("SELECT COUNT(*) FROM workflow_runs").Scan(&totalCount)
	if err != nil {
		// If no rows found (shouldn't happen with COUNT(*) but handle it anyway), return 0
		if err == sql.ErrNoRows {
			return []models.WorkflowRun{}, 0, nil
		}
		return nil, 0, err
	}

	// Get paginated results
	rows, err := DB.Query("SELECT id, name, status, repository, html_url, display_title, conclusion, created_at, run_started_at, updated_at FROM workflow_runs ORDER BY created_at DESC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var runs []models.WorkflowRun
	var run models.WorkflowRun
	var startedAt sql.NullTime
	var updatedAt sql.NullTime
	for rows.Next() {
		if err := rows.Scan(&run.ID, &run.Name, &run.Status, &run.RepositoryName, &run.HtmlUrl, &run.DisplayTitle, &run.Conclusion, &run.CreatedAt, &startedAt, &updatedAt); err != nil {
			return nil, 0, err
		}
		run.RunStartedAt = startedAt.Time
		run.UpdatedAt = updatedAt.Time
		runs = append(runs, run)
	}

	return runs, totalCount, nil
}

func (db *DBWrapper) GetJobsByLabel(page int, limit int) ([]models.LabelMetrics, int, error) {
	// Calculate offset
	offset := (page - 1) * limit

	// Count the total number of distinct label+runner_type combinations
	var total int
	err := DB.QueryRow("SELECT COUNT(*) FROM (SELECT DISTINCT labels, runner_type FROM workflow_jobs) AS distinct_combinations").Scan(&total)
	if err != nil {
		// If no rows found (shouldn't happen with COUNT(*) but handle it anyway), return 0
		if err == sql.ErrNoRows {
			return []models.LabelMetrics{}, 0, nil
		}
		return nil, 0, err
	}

	query := `
        SELECT 
            labels,
			runner_type,
            COUNT(CASE WHEN status = 'queued' THEN 1 END) as queued_count,
            COUNT(CASE WHEN status = 'in_progress' THEN 1 END) as running_count,
            COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_count,
            COUNT(CASE WHEN status = 'cancelled' THEN 1 END) as cancelled_count,
            COUNT(*) as total_count
        FROM workflow_jobs 
        GROUP BY labels, runner_type
		ORDER BY total_count DESC
		LIMIT $1 OFFSET $2
    `

	rows, err := DB.Query(query, limit, offset)
	if err != nil {
		return nil, total, err
	}
	defer rows.Close()

	// type
	var labelCounts []models.LabelMetrics
	var labels []string
	var runnerType string
	var queuedCount, runningCount, completedCount, cancelledCount, totalCount int
	for rows.Next() {
		if err := rows.Scan(pq.Array(&labels), &runnerType, &queuedCount, &runningCount, &completedCount, &cancelledCount, &totalCount); err != nil {
			return nil, total, err
		}

		labelCounts = append(labelCounts, models.LabelMetrics{
			Labels:         labels,
			RunnerType:     models.RunnerType(runnerType),
			QueuedCount:    queuedCount,
			RunningCount:   runningCount,
			CompletedCount: completedCount,
			CancelledCount: cancelledCount,
			TotalCount:     totalCount,
		})
	}

	return labelCounts, total, nil
}

func (db *DBWrapper) GetWorkflowJobsByRunID(runID int64) ([]models.WorkflowJob, error) {

	rows, err := DB.Query("SELECT id, name, run_id, status, runner_type, labels, conclusion, created_at, started_at, completed_at FROM workflow_jobs WHERE run_id = $1 ORDER BY created_at DESC", runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.WorkflowJob
	var job models.WorkflowJob
	var startedAt sql.NullTime
	var completed_at sql.NullTime
	for rows.Next() {
		if err := rows.Scan(&job.ID, &job.Name, &job.RunID, &job.Status, &job.RunnerType, pq.Array(&job.Labels), &job.Conclusion, &job.CreatedAt, &startedAt, &completed_at); err != nil {
			return nil, err
		}
		job.StartedAt = startedAt.Time
		job.CompletedAt = completed_at.Time
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetWorkflowJobByID retrieves a single workflow job by its ID
func (db *DBWrapper) GetWorkflowJobByID(jobID int64) (models.WorkflowJob, error) {
	var job models.WorkflowJob
	var labels []string
	var runnerType string
	var startedAt sql.NullTime
	var completedAt sql.NullTime

	err := DB.QueryRow(`
		SELECT id, name, run_id, status, runner_type, labels, conclusion, 
			   created_at, started_at, completed_at 
		FROM workflow_jobs 
		WHERE id = $1`, jobID).Scan(
		&job.ID, &job.Name, &job.RunID, &job.Status, &runnerType,
		pq.Array(&labels), &job.Conclusion, &job.CreatedAt,
		&startedAt, &completedAt)

	if err != nil {
		// Return empty job with default status if not found
		if err == sql.ErrNoRows {
			return models.WorkflowJob{Status: ""}, nil
		}
		return models.WorkflowJob{}, err
	}

	job.Labels = labels
	job.RunnerType = models.RunnerType(runnerType)
	job.StartedAt = startedAt.Time
	job.CompletedAt = completedAt.Time

	return job, nil
}

// CleanupOldData removes workflow runs and jobs older than the retention period
// Returns the number of deleted workflow runs and workflow jobs
func (db *DBWrapper) CleanupOldData(retentionPeriod time.Duration) (int64, int64, int64, error) {
	// Calculate the cutoff time
	cutoffTime := time.Now().Add(-retentionPeriod)

	// Start a transaction to ensure both deletions are atomic
	tx, err := DB.Begin()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete old workflow jobs first (due to foreign key constraint)
	jobResult, err := tx.Exec("DELETE FROM workflow_jobs WHERE created_at < $1", cutoffTime)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to delete old workflow jobs: %w", err)
	}

	deletedJobs, err := jobResult.RowsAffected()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get affected jobs count: %w", err)
	}

	// Delete old workflow runs
	runResult, err := tx.Exec("DELETE FROM workflow_runs WHERE created_at < $1", cutoffTime)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to delete old workflow runs: %w", err)
	}

	deletedRuns, err := runResult.RowsAffected()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get affected runs count: %w", err)
	}

	eventResult, err := tx.Exec("DELETE FROM webhook_events WHERE processed_at < $1", cutoffTime)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to delete old webhook events: %w", err)
	}

	deletedEvents, err := eventResult.RowsAffected()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get affected events count: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to commit cleanup transaction: %w", err)
	}

	return deletedRuns, deletedJobs, deletedEvents, nil
}

func (db *DBWrapper) GetCurrentJobCounts() (map[string]map[string]int, error) {
	query := `
        SELECT 
            runner_type,
            SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END) as queued_count,
            SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END) as in_progress_count
        FROM workflow_jobs 
        GROUP BY runner_type
        ORDER BY runner_type
    `

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]map[string]int)

	for rows.Next() {
		var runnerType string
		var queuedCount, inProgressCount int

		if err := rows.Scan(&runnerType, &queuedCount, &inProgressCount); err != nil {
			return nil, err
		}

		result[runnerType] = map[string]int{
			"queued":  queuedCount,
			"running": inProgressCount,
		}
	}

	return result, nil
}
