package config

import (
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	// Clear environment variables before test
	os.Clearenv()

	t.Run("with default values", func(t *testing.T) {
		config, err := NewConfig()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if config.Vars.Port != "8080" {
			t.Errorf("Expected Port to be 8080, got %s", config.Vars.Port)
		}
		if config.Vars.DatabasePath != "./data/live-actions.db" {
			t.Errorf("Expected DatabasePath to be ./data/live-actions.db, got %s", config.Vars.DatabasePath)
		}
		if config.Vars.LogLevel != "info" {
			t.Errorf("Expected LogLevel to be info, got %s", config.Vars.LogLevel)
		}
	})

	t.Run("with custom environment values", func(t *testing.T) {
		// Set custom environment variables
		os.Setenv("WEBHOOK_SECRET", "test-secret")
		os.Setenv("PORT", "3000")
		os.Setenv("DATABASE_PATH", "test.db")
		os.Setenv("LOG_LEVEL", "debug")

		config, err := NewConfig()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if config.Vars.WebhookSecret != "test-secret" {
			t.Errorf("Expected WebhookSecret to be test-secret, got %s", config.Vars.WebhookSecret)
		}
		if config.Vars.Port != "3000" {
			t.Errorf("Expected Port to be 3000, got %s", config.Vars.Port)
		}
		if config.Vars.DatabasePath != "test.db" {
			t.Errorf("Expected DatabasePath to be test.db, got %s", config.Vars.DatabasePath)
		}
		if config.Vars.LogLevel != "debug" {
			t.Errorf("Expected LogLevel to be debug, got %s", config.Vars.LogLevel)
		}
	})

	t.Run("production without webhook secret", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("ENVIRONMENT", "production")

		_, err := NewConfig()
		if err == nil {
			t.Error("Expected error for missing WEBHOOK_SECRET in production")
		}
	})
}

func TestGetDatabasePath(t *testing.T) {
	os.Clearenv()

	t.Run("with default values", func(t *testing.T) {
		config, err := NewConfig()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := "./data/live-actions.db"
		if p := config.GetDatabasePath(); p != expected {
			t.Errorf("Expected DatabasePath %s, got %s", expected, p)
		}
	})

	t.Run("with custom values", func(t *testing.T) {
		os.Setenv("DATABASE_PATH", "custom.db")

		config, err := NewConfig()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := "custom.db"
		if p := config.GetDatabasePath(); p != expected {
			t.Errorf("Expected DatabasePath %s, got %s", expected, p)
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

func TestNewConfigGitHubRepos(t *testing.T) {
	setWebSocketConfigEnv := func(t *testing.T) {
		t.Helper()
		t.Setenv("ENVIRONMENT", "development")
		t.Setenv("WEBHOOK_TRANSPORT", "websocket")
		t.Setenv("GITHUB_TOKEN", "test-token")
		t.Setenv("GITHUB_REPO", "")
		t.Setenv("GITHUB_REPOS", "")
		t.Setenv("GITHUB_ORG", "")
		t.Setenv("GITHUB_ENTERPRISE", "")
	}

	t.Run("accepts multiple repositories", func(t *testing.T) {
		setWebSocketConfigEnv(t)
		t.Setenv("GITHUB_REPOS", "owner/one, owner/two")

		cfg, err := NewConfig()
		if err != nil {
			t.Fatalf("NewConfig() error = %v", err)
		}
		if cfg.Vars.GitHubRepos != "owner/one, owner/two" {
			t.Fatalf("GitHubRepos = %q, want the configured list", cfg.Vars.GitHubRepos)
		}
	})

	t.Run("rejects malformed repository entries", func(t *testing.T) {
		for _, repos := range []string{"owner/one,", "owner/", "/one", "owner/one/extra", "owner/ one"} {
			t.Run(repos, func(t *testing.T) {
				setWebSocketConfigEnv(t)
				t.Setenv("GITHUB_REPOS", repos)

				if _, err := NewConfig(); err == nil {
					t.Fatalf("NewConfig() accepted malformed GITHUB_REPOS %q", repos)
				}
			})
		}
	})

	t.Run("rejects duplicate repositories", func(t *testing.T) {
		setWebSocketConfigEnv(t)
		t.Setenv("GITHUB_REPOS", "owner/one, OWNER/ONE")

		if _, err := NewConfig(); err == nil {
			t.Fatal("NewConfig() accepted duplicate repositories")
		}
	})

	t.Run("rejects multiple repository target settings", func(t *testing.T) {
		setWebSocketConfigEnv(t)
		t.Setenv("GITHUB_REPOS", "owner/one,owner/two")
		t.Setenv("GITHUB_REPO", "owner/one")

		if _, err := NewConfig(); err == nil {
			t.Fatal("NewConfig() accepted both GITHUB_REPOS and GITHUB_REPO")
		}
	})
}
