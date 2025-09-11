package database

import (
	"testing"
	"time"

	"github.com/gateixeira/live-actions/models"
	"github.com/stretchr/testify/assert"
)

func TestAddOrUpdateJob_NewJob(t *testing.T) {
	mockDB := &MockDatabase{}

	job := models.WorkflowJob{
		ID:         123,
		Name:       "test-job",
		Status:     models.JobStatusQueued,
		RunnerType: models.RunnerTypeGitHubHosted,
		Labels:     []string{"ubuntu-latest"},
		CreatedAt:  time.Now(),
		RunID:      456,
	}

	eventTime := time.Now()

	// Mock should return that the job was updated
	mockDB.On("AddOrUpdateJob", job, eventTime).Return(true, nil)

	updated, err := mockDB.AddOrUpdateJob(job, eventTime)

	assert.NoError(t, err)
	assert.True(t, updated)
	mockDB.AssertExpectations(t)
}

func TestAddOrUpdateJob_RejectOlderEvent(t *testing.T) {
	mockDB := &MockDatabase{}

	job := models.WorkflowJob{
		ID:         123,
		Name:       "test-job",
		Status:     models.JobStatusQueued, // Trying to set to queued
		RunnerType: models.RunnerTypeGitHubHosted,
		Labels:     []string{"ubuntu-latest"},
		CreatedAt:  time.Now(),
		RunID:      456,
	}

	// Event timestamp is older than when job was completed
	eventTime := time.Now().Add(-10 * time.Minute)

	// Mock should return that the job was NOT updated due to atomicity
	mockDB.On("AddOrUpdateJob", job, eventTime).Return(false, nil)

	updated, err := mockDB.AddOrUpdateJob(job, eventTime)

	assert.NoError(t, err)
	assert.False(t, updated, "Should reject older event when job is already in terminal state")
	mockDB.AssertExpectations(t)
}

func TestAddOrUpdateRun_NewRun(t *testing.T) {
	mockDB := &MockDatabase{}

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

	// Mock should return that the run was updated
	mockDB.On("AddOrUpdateRun", run, eventTime).Return(true, nil)

	updated, err := mockDB.AddOrUpdateRun(run, eventTime)

	assert.NoError(t, err)
	assert.True(t, updated)
	mockDB.AssertExpectations(t)
}

func TestAddOrUpdateRun_RejectOlderEvent(t *testing.T) {
	mockDB := &MockDatabase{}

	run := models.WorkflowRun{
		ID:             789,
		Name:           "test-workflow",
		Status:         models.JobStatusInProgress, // Trying to set to in_progress
		RepositoryName: "test-repo",
		HtmlUrl:        "https://github.com/test/repo",
		DisplayTitle:   "Test Run",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Event timestamp is older than when run was completed
	eventTime := time.Now().Add(-10 * time.Minute)

	// Mock should return that the run was NOT updated due to atomicity
	mockDB.On("AddOrUpdateRun", run, eventTime).Return(false, nil)

	updated, err := mockDB.AddOrUpdateRun(run, eventTime)

	assert.NoError(t, err)
	assert.False(t, updated, "Should reject older event when run is already in terminal state")
	mockDB.AssertExpectations(t)
}
