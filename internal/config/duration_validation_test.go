package config

import (
	"strings"
	"testing"
)

func setValidDurationTestEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("WEBHOOK_TRANSPORT", "http")
	t.Setenv("DATA_RETENTION_DAYS", "30")
	t.Setenv("CLEANUP_INTERVAL_HOURS", "24")
	t.Setenv("STALE_JOB_THRESHOLD_HOURS", "24")
}

func TestNewConfigRejectsInvalidDurationValues(t *testing.T) {
	for _, envName := range []string{
		"DATA_RETENTION_DAYS",
		"CLEANUP_INTERVAL_HOURS",
		"STALE_JOB_THRESHOLD_HOURS",
	} {
		for _, value := range []string{"0", "-1", "not-a-number"} {
			t.Run(envName+"="+value, func(t *testing.T) {
				setValidDurationTestEnvironment(t)
				t.Setenv(envName, value)

				_, err := NewConfig()
				if err == nil || !strings.Contains(err.Error(), envName) {
					t.Fatalf("NewConfig() error = %v, want an error naming %s", err, envName)
				}
			})
		}
	}
}

func TestNewConfigRejectsDurationOverflow(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "DATA_RETENTION_DAYS", value: "106752"},
		{name: "CLEANUP_INTERVAL_HOURS", value: "2562048"},
		{name: "STALE_JOB_THRESHOLD_HOURS", value: "2562048"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidDurationTestEnvironment(t)
			t.Setenv(tt.name, tt.value)

			_, err := NewConfig()
			if err == nil || !strings.Contains(err.Error(), "maximum duration") {
				t.Fatalf("NewConfig() error = %v, want a maximum-duration validation error", err)
			}
		})
	}
}

func TestNewConfigAcceptsPositiveDurationValues(t *testing.T) {
	setValidDurationTestEnvironment(t)
	t.Setenv("DATA_RETENTION_DAYS", "1")
	t.Setenv("CLEANUP_INTERVAL_HOURS", "2")
	t.Setenv("STALE_JOB_THRESHOLD_HOURS", "3")

	config, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}
	if config.Vars.DataRetentionDays != 1 || config.Vars.CleanupIntervalHours != 2 || config.Vars.StaleJobThresholdHours != 3 {
		t.Fatalf("unexpected duration values: %+v", config.Vars)
	}
}
