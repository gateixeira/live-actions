package database

import (
	"context"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completedRunnerJob builds a completed job that ran on the given runner for
// dur and finished at completedAt.
func completedRunnerJob(id, runID int64, runner, group, conclusion string, completedAt time.Time, dur time.Duration) models.WorkflowJob {
	startedAt := completedAt.Add(-dur)
	return models.WorkflowJob{
		ID:              id,
		Name:            "build",
		Status:          models.JobStatusCompleted,
		Labels:          []string{"self-hosted"},
		Conclusion:      conclusion,
		CreatedAt:       startedAt.Add(-time.Minute),
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
		RunID:           runID,
		RunnerName:      runner,
		RunnerGroupName: group,
	}
}

func inProgressRunnerJob(id, runID int64, runner, group string, startedAt time.Time) models.WorkflowJob {
	return models.WorkflowJob{
		ID:              id,
		Name:            "build",
		Status:          models.JobStatusInProgress,
		Labels:          []string{"self-hosted"},
		CreatedAt:       startedAt.Add(-time.Minute),
		StartedAt:       startedAt,
		RunID:           runID,
		RunnerName:      runner,
		RunnerGroupName: group,
	}
}

func findRunner(stats []models.RunnerStats, name string) *models.RunnerStats {
	for i := range stats {
		if stats[i].RunnerName == name {
			return &stats[i]
		}
	}
	return nil
}

func TestGetRunnerStats_AggregatesPerRunner(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	runRepo := newRun(1, models.JobStatusCompleted)
	runRepo.RepositoryName = "owner/repo"
	runOther := newRun(2, models.JobStatusCompleted)
	runOther.RepositoryName = "owner/other"
	for _, run := range []models.WorkflowRun{runRepo, runOther} {
		_, err := db.AddOrUpdateRun(ctx, run, now)
		require.NoError(t, err)
	}

	noRunner := completedRunnerJob(10, 1, "", "", "cancelled", now.Add(-time.Minute), 0)
	noRunner.StartedAt = time.Time{}

	jobs := []models.WorkflowJob{
		// runner-a, owner/repo: one job per conclusion bucket within the last day.
		completedRunnerJob(1, 1, "runner-a", "Default", "success", now.Add(-10*time.Minute), 60*time.Second),
		completedRunnerJob(2, 1, "runner-a", "Default", "failure", now.Add(-20*time.Minute), 120*time.Second),
		completedRunnerJob(3, 1, "runner-a", "Default", "timed_out", now.Add(-30*time.Minute), 180*time.Second),
		completedRunnerJob(4, 1, "runner-a", "Default", "cancelled", now.Add(-40*time.Minute), 30*time.Second),
		completedRunnerJob(5, 1, "runner-a", "Default", "skipped", now.Add(-50*time.Minute), 10*time.Second),
		// Outside the day window, inside the week window.
		completedRunnerJob(6, 1, "runner-a", "Default", "success", now.Add(-72*time.Hour), 600*time.Second),
		// runner-b, owner/other: one completed job and one running now.
		completedRunnerJob(7, 2, "runner-b", "GPU", "success", now.Add(-5*time.Minute), 90*time.Second),
		inProgressRunnerJob(8, 2, "runner-b", "GPU", now.Add(-2*time.Minute)),
		// runner-c, owner/repo: only a running job, nothing completed.
		inProgressRunnerJob(9, 1, "runner-c", "Default", now.Add(-time.Minute)),
		// Never picked up by a runner: must not appear.
		noRunner,
	}
	for _, job := range jobs {
		updated, err := db.AddOrUpdateJob(ctx, job, now)
		require.NoError(t, err)
		require.True(t, updated, "job %d should be stored", job.ID)
	}

	t.Run("day across all repositories", func(t *testing.T) {
		stats, err := db.GetRunnerStats(ctx, 24*time.Hour, "")
		require.NoError(t, err)
		require.Len(t, stats, 3)
		assert.Equal(t, "runner-a", stats[0].RunnerName)
		assert.Equal(t, "runner-b", stats[1].RunnerName)
		assert.Equal(t, "runner-c", stats[2].RunnerName)

		a := stats[0]
		assert.Equal(t, "Default", a.RunnerGroupName)
		assert.Equal(t, 5, a.TotalJobs)
		assert.Equal(t, 1, a.Succeeded)
		assert.Equal(t, 2, a.Failed, "failure and timed_out both count as failed")
		assert.Equal(t, 1, a.Cancelled)
		assert.Equal(t, 1, a.Other)
		assert.InDelta(t, 0.4, a.FailureRate, 1e-9)
		assert.InDelta(t, 80, a.AvgDurationSeconds, 0.01)
		assert.Equal(t, 0, a.Running)
		require.NotNil(t, a.LastJobAt)
		assert.True(t, now.Add(-10*time.Minute).Equal(*a.LastJobAt), "last_job_at = %s", a.LastJobAt)

		b := stats[1]
		assert.Equal(t, "GPU", b.RunnerGroupName)
		assert.Equal(t, 1, b.TotalJobs)
		assert.Equal(t, 1, b.Succeeded)
		assert.Equal(t, 0.0, b.FailureRate)
		assert.InDelta(t, 90, b.AvgDurationSeconds, 0.01)
		assert.Equal(t, 1, b.Running)
		require.NotNil(t, b.LastJobAt)
		assert.True(t, now.Add(-5*time.Minute).Equal(*b.LastJobAt))

		c := stats[2]
		assert.Equal(t, "Default", c.RunnerGroupName)
		assert.Equal(t, 0, c.TotalJobs)
		assert.Equal(t, 0.0, c.FailureRate)
		assert.Equal(t, 0.0, c.AvgDurationSeconds)
		assert.Equal(t, 1, c.Running)
		assert.Nil(t, c.LastJobAt)
	})

	t.Run("week includes older jobs", func(t *testing.T) {
		stats, err := db.GetRunnerStats(ctx, 7*24*time.Hour, "")
		require.NoError(t, err)
		a := findRunner(stats, "runner-a")
		require.NotNil(t, a)
		assert.Equal(t, 6, a.TotalJobs)
		assert.Equal(t, 2, a.Succeeded)
		assert.InDelta(t, 2.0/6.0, a.FailureRate, 1e-9)
		assert.InDelta(t, 1000.0/6.0, a.AvgDurationSeconds, 0.01)
		require.NotNil(t, a.LastJobAt)
		assert.True(t, now.Add(-10*time.Minute).Equal(*a.LastJobAt))
	})

	t.Run("repository filter", func(t *testing.T) {
		stats, err := db.GetRunnerStats(ctx, 24*time.Hour, "owner/other")
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, "runner-b", stats[0].RunnerName)
		assert.Equal(t, 1, stats[0].TotalJobs)
		assert.Equal(t, 1, stats[0].Running)

		stats, err = db.GetRunnerStats(ctx, 24*time.Hour, "owner/repo")
		require.NoError(t, err)
		require.Len(t, stats, 2)
		assert.Equal(t, "runner-a", stats[0].RunnerName)
		assert.Equal(t, "runner-c", stats[1].RunnerName)
	})

	t.Run("unknown repository returns empty slice", func(t *testing.T) {
		stats, err := db.GetRunnerStats(ctx, 24*time.Hour, "owner/missing")
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Empty(t, stats)
	})
}

func TestAddOrUpdateJob_KeepsRunnerWhenLaterEventOmitsIt(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	job := newJob(1, models.JobStatusQueued)
	_, err := db.AddOrUpdateJob(ctx, job, now)
	require.NoError(t, err)

	got, err := db.GetWorkflowJobByID(ctx, 1)
	require.NoError(t, err)
	assert.Empty(t, got.RunnerName, "queued job has no runner yet")

	job.Status = models.JobStatusInProgress
	job.StartedAt = now
	job.RunnerName = "runner-a"
	job.RunnerGroupName = "Default"
	updated, err := db.AddOrUpdateJob(ctx, job, now)
	require.NoError(t, err)
	require.True(t, updated)

	// The completed event arrives without runner fields; the stored ones stay.
	job.Status = models.JobStatusCompleted
	job.Conclusion = "success"
	job.CompletedAt = now.Add(time.Minute)
	job.RunnerName = ""
	job.RunnerGroupName = ""
	updated, err = db.AddOrUpdateJob(ctx, job, now)
	require.NoError(t, err)
	require.True(t, updated)

	got, err = db.GetWorkflowJobByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, models.JobStatusCompleted, got.Status)
	assert.Equal(t, "runner-a", got.RunnerName)
	assert.Equal(t, "Default", got.RunnerGroupName)

	jobs, err := db.GetWorkflowJobsByRunID(ctx, job.RunID)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "runner-a", jobs[0].RunnerName)
	assert.Equal(t, "Default", jobs[0].RunnerGroupName)
}

func TestGetWorkflowJob_NullRunnerColumns(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Rows written before the runner columns existed have NULL in them.
	w, ok := db.(*DBWrapper)
	require.True(t, ok)
	_, err := w.writeDB.ExecContext(ctx,
		`INSERT INTO workflow_jobs (id, name, run_id, status, labels, conclusion, created_at)
		VALUES (?, 'legacy', ?, 'completed', '[]', 'success', ?)`,
		42, 420, time.Now().UTC().Format(time.RFC3339))
	require.NoError(t, err)

	got, err := db.GetWorkflowJobByID(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, "legacy", got.Name)
	assert.Empty(t, got.RunnerName)
	assert.Empty(t, got.RunnerGroupName)

	jobs, err := db.GetWorkflowJobsByRunID(ctx, 420)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Empty(t, jobs[0].RunnerName)
}
