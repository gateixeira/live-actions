package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDB returns a DBWrapper backed by a fresh on-disk SQLite database in
// the test's temp directory, so each test gets full isolation including
// schema migrations.
func newTestDB(t *testing.T) DatabaseInterface {
	t.Helper()
	logger.InitLogger("error")
	dir := t.TempDir()
	w, r, err := InitDB(filepath.Join(dir, "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = w.Close()
		_ = r.Close()
	})
	return NewDBWrapper(w, r)
}

func newJob(id int64, status models.JobStatus) models.WorkflowJob {
	return models.WorkflowJob{
		ID:        id,
		Name:      "test-job",
		Status:    status,
		Labels:    []string{"ubuntu-latest"},
		CreatedAt: time.Now(),
		RunID:     id * 10,
	}
}

func newRun(id int64, status models.JobStatus) models.WorkflowRun {
	return models.WorkflowRun{
		ID:             id,
		Name:           "test-run",
		Status:         status,
		RepositoryName: "owner/repo",
		CreatedAt:      time.Now(),
	}
}

// ---- AddOrUpdateJob transition rules ------------------------------------

func TestAddOrUpdateJob_AcceptsLifecycleProgression(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	for _, status := range []models.JobStatus{
		models.JobStatusQueued,
		models.JobStatusInProgress,
		models.JobStatusCompleted,
	} {
		updated, err := db.AddOrUpdateJob(ctx, newJob(1, status), time.Now())
		require.NoError(t, err)
		assert.True(t, updated, "status %s should be applied", status)
	}

	got, err := db.GetWorkflowJobByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, models.JobStatusCompleted, got.Status)
}

func TestAddOrUpdateJob_RejectsLowerPriorityAfterInProgress(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.AddOrUpdateJob(ctx, newJob(1, models.JobStatusInProgress), time.Now())
	require.NoError(t, err)

	// Late-arriving `queued` event must NOT downgrade an in_progress job.
	updated, err := db.AddOrUpdateJob(ctx, newJob(1, models.JobStatusQueued), time.Now())
	require.NoError(t, err)
	assert.False(t, updated)

	got, err := db.GetWorkflowJobByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, models.JobStatusInProgress, got.Status)
}

func TestAddOrUpdateJob_RejectsCancelledOverwritingCompleted(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.AddOrUpdateJob(ctx, newJob(1, models.JobStatusCompleted), time.Now())
	require.NoError(t, err)

	// `cancelled` and `completed` share priority 5; the existing terminal
	// state must be preserved.
	updated, err := db.AddOrUpdateJob(ctx, newJob(1, models.JobStatusCancelled), time.Now())
	require.NoError(t, err)
	assert.False(t, updated)

	got, err := db.GetWorkflowJobByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, models.JobStatusCompleted, got.Status)
}

func TestAddOrUpdateJob_StaleIsTerminalAndBlocksFurtherWrites(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.AddOrUpdateJob(ctx, newJob(1, models.JobStatusStale), time.Now())
	require.NoError(t, err)

	for _, status := range []models.JobStatus{
		models.JobStatusInProgress,
		models.JobStatusQueued,
		models.JobStatusCompleted,
	} {
		updated, err := db.AddOrUpdateJob(ctx, newJob(1, status), time.Now())
		require.NoError(t, err)
		assert.False(t, updated, "stale must reject incoming %s", status)
	}

	got, err := db.GetWorkflowJobByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, models.JobStatusStale, got.Status)
}

func TestAddOrUpdateJob_IdempotentTerminalReplay(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.AddOrUpdateJob(ctx, newJob(1, models.JobStatusCompleted), time.Now())
	require.NoError(t, err)

	// Replaying the same terminal status is a no-op (false, nil) — not an
	// error, and the row remains unchanged.
	updated, err := db.AddOrUpdateJob(ctx, newJob(1, models.JobStatusCompleted), time.Now())
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestAddOrUpdateJob_RejectsUnknownStatus(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	updated, err := db.AddOrUpdateJob(ctx, newJob(1, models.JobStatus("nonsense")), time.Now())
	require.NoError(t, err)
	assert.False(t, updated)

	// Row must not exist.
	got, err := db.GetWorkflowJobByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(0), got.ID)
}

// ---- AddOrUpdateRun transition rules ------------------------------------

func TestAddOrUpdateRun_RejectsLowerPriorityAfterInProgress(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.AddOrUpdateRun(ctx, newRun(1, models.JobStatusInProgress), time.Now())
	require.NoError(t, err)

	updated, err := db.AddOrUpdateRun(ctx, newRun(1, models.JobStatusRequested), time.Now())
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestAddOrUpdateRun_RejectsCancelledOverwritingCompleted(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.AddOrUpdateRun(ctx, newRun(1, models.JobStatusCompleted), time.Now())
	require.NoError(t, err)

	updated, err := db.AddOrUpdateRun(ctx, newRun(1, models.JobStatusCancelled), time.Now())
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestAddOrUpdateRun_RejectsUnknownStatus(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	updated, err := db.AddOrUpdateRun(ctx, newRun(1, models.JobStatus("queued")), time.Now())
	// "queued" is NOT a valid run status (only requested/in_progress/
	// completed/cancelled). Reject without error.
	require.NoError(t, err)
	assert.False(t, updated)
}

// ---- Status priority helpers --------------------------------------------

func TestJobStatusPriority_OrdersLifecycleMonotonically(t *testing.T) {
	order := []models.JobStatus{
		models.JobStatusWaiting,
		models.JobStatusQueued,
		models.JobStatusRequested,
		models.JobStatusInProgress,
		models.JobStatusCompleted,
	}
	prev, _ := models.JobStatusPriority(order[0])
	for _, s := range order[1:] {
		p, ok := models.JobStatusPriority(s)
		require.True(t, ok)
		assert.Greater(t, p, prev, "priority of %s must be > previous", s)
		prev = p
	}

	// Unknown sentinel is rejected.
	_, ok := models.JobStatusPriority(models.JobStatus("xxx"))
	assert.False(t, ok)
}
