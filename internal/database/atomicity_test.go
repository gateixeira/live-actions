package database

import (
	"context"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAddOrUpdateJob_NewJob(t *testing.T) {
	mockDB := &MockDatabase{}
	ctx := context.Background()

	job := models.WorkflowJob{
		ID:        123,
		Name:      "test-job",
		Status:    models.JobStatusQueued,
		Labels:    []string{"ubuntu-latest"},
		CreatedAt: time.Now(),
		RunID:     456,
	}

	eventTime := time.Now()

	mockDB.On("AddOrUpdateJob", mock.Anything, job, eventTime).Return(true, nil)

	updated, err := mockDB.AddOrUpdateJob(ctx, job, eventTime)

	assert.NoError(t, err)
	assert.True(t, updated)
	mockDB.AssertExpectations(t)
}

func TestAddOrUpdateJob_RejectOlderEvent(t *testing.T) {
	mockDB := &MockDatabase{}
	ctx := context.Background()

	job := models.WorkflowJob{
		ID:        123,
		Name:      "test-job",
		Status:    models.JobStatusQueued,
		Labels:    []string{"ubuntu-latest"},
		CreatedAt: time.Now(),
		RunID:     456,
	}

	eventTime := time.Now().Add(-10 * time.Minute)

	mockDB.On("AddOrUpdateJob", mock.Anything, job, eventTime).Return(false, nil)

	updated, err := mockDB.AddOrUpdateJob(ctx, job, eventTime)

	assert.NoError(t, err)
	assert.False(t, updated, "Should reject older event when job is already in terminal state")
	mockDB.AssertExpectations(t)
}

func TestAddOrUpdateRun_NewRun(t *testing.T) {
	mockDB := &MockDatabase{}
	ctx := context.Background()

	run := models.WorkflowRun{
		ID:             789,
		Name:           "test-workflow",
		Status:         models.JobStatusInProgress,
		RepositoryName: "test-repo",
		HtmlUrl:        "https://github.com/test/repo",
		DisplayTitle:   "Test Run",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	eventTime := time.Now()

	mockDB.On("AddOrUpdateRun", mock.Anything, run, eventTime).Return(true, nil)

	updated, err := mockDB.AddOrUpdateRun(ctx, run, eventTime)

	assert.NoError(t, err)
	assert.True(t, updated)
	mockDB.AssertExpectations(t)
}

func TestAddOrUpdateRun_RejectOlderEvent(t *testing.T) {
	mockDB := &MockDatabase{}
	ctx := context.Background()

	run := models.WorkflowRun{
		ID:             789,
		Name:           "test-workflow",
		Status:         models.JobStatusInProgress,
		RepositoryName: "test-repo",
		HtmlUrl:        "https://github.com/test/repo",
		DisplayTitle:   "Test Run",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	eventTime := time.Now().Add(-10 * time.Minute)

	mockDB.On("AddOrUpdateRun", mock.Anything, run, eventTime).Return(false, nil)

	updated, err := mockDB.AddOrUpdateRun(ctx, run, eventTime)

	assert.NoError(t, err)
	assert.False(t, updated, "Should reject older event when run is already in terminal state")
	mockDB.AssertExpectations(t)
}
