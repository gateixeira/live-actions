package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfig(t *testing.T) {
	// Clear environment variables before test
	os.Clearenv()

	// Set a test config path that exists relative to the project root
	// Since tests run from the package directory, we need to go up to find the config
	testConfigPath := filepath.Join("..", "..", "config", "runner_types.json")
	os.Setenv("RUNNER_TYPE_CONFIG_PATH", testConfigPath)

	t.Run("with default values", func(t *testing.T) {
		config := NewConfig()

		if config.Vars.Port != "8080" {
			t.Errorf("Expected Port to be 8080, got %s", config.Vars.Port)
		}
		if config.Vars.DatabaseURL != "postgresql://postgres:@localhost:5432/live-actions?sslmode=disable" {
			t.Errorf("Expected DatabaseURL to be postgresql://postgres:@localhost:5432/live-actions?sslmode=disable, got %s", config.Vars.DatabaseURL)
		}
		if config.Vars.LogLevel != "info" {
			t.Errorf("Expected LogLevel to be info, got %s", config.Vars.LogLevel)
		}
	})

	t.Run("with custom environment values", func(t *testing.T) {
		// Set a test config path that exists relative to the project root
		testConfigPath := filepath.Join("..", "..", "config", "runner_types.json")

		// Set custom environment variables
		os.Setenv("WEBHOOK_SECRET", "test-secret")
		os.Setenv("PORT", "3000")
		os.Setenv("DATABASE_URL", "postgresql://test-user:test-password@test-host:5433/test-db?sslmode=require")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("RUNNER_TYPE_CONFIG_PATH", testConfigPath)

		config := NewConfig()

		if config.Vars.WebhookSecret != "test-secret" {
			t.Errorf("Expected WebhookSecret to be test-secret, got %s", config.Vars.WebhookSecret)
		}
		if config.Vars.Port != "3000" {
			t.Errorf("Expected Port to be 3000, got %s", config.Vars.Port)
		}
		if config.Vars.DatabaseURL != "postgresql://test-user:test-password@test-host:5433/test-db?sslmode=require" {
			t.Errorf("Expected DatabaseURL to be postgresql://test-user:test-password@test-host:5433/test-db?sslmode=require, got %s", config.Vars.DatabaseURL)
		}
		if config.Vars.LogLevel != "debug" {
			t.Errorf("Expected LogLevel to be debug, got %s", config.Vars.LogLevel)
		}
	})
}

func TestGetDSN(t *testing.T) {
	os.Clearenv()

	t.Run("with default values", func(t *testing.T) {
		// Set a test config path that exists relative to the project root
		testConfigPath := filepath.Join("..", "..", "config", "runner_types.json")
		os.Setenv("RUNNER_TYPE_CONFIG_PATH", testConfigPath)

		config := NewConfig()
		expected := "postgresql://postgres:@localhost:5432/live-actions?sslmode=disable"
		if dsn := config.GetDSN(); dsn != expected {
			t.Errorf("Expected DSN %s, got %s", expected, dsn)
		}
	})

	t.Run("with custom values", func(t *testing.T) {
		// Set a test config path that exists relative to the project root
		testConfigPath := filepath.Join("..", "..", "config", "runner_types.json")

		os.Setenv("DATABASE_URL", "postgresql://test-user:test-password@test-host:5433/test-db?sslmode=require")
		os.Setenv("RUNNER_TYPE_CONFIG_PATH", testConfigPath)

		config := NewConfig()
		expected := "postgresql://test-user:test-password@test-host:5433/test-db?sslmode=require"
		if dsn := config.GetDSN(); dsn != expected {
			t.Errorf("Expected DSN %s, got %s", expected, dsn)
		}
	})
}

func TestGetEnvOrDefault(t *testing.T) {
	os.Clearenv()

	tests := []struct {
		name         string
		key          string
		defaultVal   string
		envVal       string
		expected     string
		shouldSetEnv bool
	}{
		{
			name:         "returns default when env not set",
			key:          "TEST_KEY",
			defaultVal:   "default",
			envVal:       "",
			expected:     "default",
			shouldSetEnv: false,
		},
		{
			name:         "returns env value when set",
			key:          "TEST_KEY",
			defaultVal:   "default",
			envVal:       "custom",
			expected:     "custom",
			shouldSetEnv: true,
		},
		{
			name:         "handles empty default",
			key:          "TEST_KEY",
			defaultVal:   "",
			envVal:       "",
			expected:     "",
			shouldSetEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldSetEnv {
				os.Setenv(tt.key, tt.envVal)
				defer os.Unsetenv(tt.key)
			}

			result := getEnvOrDefault(tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("getEnvOrDefault() = %v, want %v", result, tt.expected)
			}
		})
	}
}
