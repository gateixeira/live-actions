package services

import (
	"context"
	"testing"
	"time"

	"github.com/gateixeira/live-actions/internal/config"
	"github.com/gateixeira/live-actions/internal/database"
	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/stretchr/testify/mock"
)

// Note: MockDatabase is now shared across packages via database.MockDatabase
// This eliminates duplication and ensures consistency across all tests

// setupTestLogger initializes the logger for testing
func setupTestLogger() {
	logger.InitLogger("error") // Use error level to reduce test output
}

func TestCleanupService_StartStop(t *testing.T) {
	// Initialize logger for testing
	setupTestLogger()
	defer logger.SyncLogger()

	// Setup
	mockDB := new(database.MockDatabase)
	config := &config.Config{
		Vars: config.Vars{
			DataRetentionDays:    1, // 1 day for faster testing
			CleanupIntervalHours: 1, // 1 hour for faster testing
		},
	}

	ctx := context.Background()
	cleanupService := NewCleanupService(config, mockDB, ctx)

	// Setup mock expectations for initial cleanup
	expectedRetention := config.GetDataRetentionDuration()
	mockDB.On("CleanupOldData", mock.Anything, expectedRetention).Return(int64(0), int64(0), int64(0), nil)

	// Start the service in a goroutine since it blocks
	done := make(chan struct{})
	go func() {
		cleanupService.Start()
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop the service
	cleanupService.Stop()

	// Wait for the service to finish
	<-done

	// Verify expectations
	mockDB.AssertExpectations(t)
}

func TestCleanupService_ContextCancellation(t *testing.T) {
	// Initialize logger for testing
	setupTestLogger()
	defer logger.SyncLogger()

	// Setup
	mockDB := new(database.MockDatabase)
	config := &config.Config{
		Vars: config.Vars{
			DataRetentionDays:    30,
			CleanupIntervalHours: 24,
		},
	}

	ctx := context.Background()
	cleanupService := NewCleanupService(config, mockDB, ctx)

	// Setup mock expectations for initial cleanup
	expectedRetention := config.GetDataRetentionDuration()
	mockDB.On("CleanupOldData", mock.Anything, expectedRetention).Return(int64(0), int64(0), int64(0), nil)

	// Start the service in a goroutine since it blocks
	done := make(chan struct{})
	go func() {
		cleanupService.Start()
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop the service (this cancels the context internally)
	cleanupService.Stop()

	// Wait for the service to finish
	<-done

	// Verify expectations
	mockDB.AssertExpectations(t)
}
