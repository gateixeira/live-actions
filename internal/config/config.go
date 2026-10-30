package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Vars struct {
	WebhookSecret          string
	Port                   string
	DatabasePath           string
	LogLevel               string
	TLSEnabled             bool
	Environment            string
	DataRetentionDays      int
	CleanupIntervalHours   int
	StaleJobThresholdHours int

	// WebhookTransport selects how GitHub deliveries reach this server.
	// "http"      (default): the server only listens on POST /webhook and
	//                        relies on GitHub being able to reach it.
	// "websocket":           additionally opens a long-lived WebSocket
	//                        relay (see internal/services/ghws). The HTTP
	//                        endpoint stays registered either way.
	WebhookTransport string
	GitHubToken      string
	GitHubHost       string
	GitHubRepo       string // owner/repo
	GitHubRepos      string // comma-separated owner/repo list
	GitHubOrg        string
	GitHubEnterprise string // enterprise slug
	GitHubEvents     string // comma-separated list, default "workflow_job,workflow_run"
}

type Config struct {
	Vars Vars
}

// NewConfig creates and initializes a new application config.
func NewConfig() (*Config, error) {
	dataRetentionDays, err := getPositiveDurationEnvOrDefault("DATA_RETENTION_DAYS", 30, 24*time.Hour)
	if err != nil {
		return nil, err
	}
	cleanupIntervalHours, err := getPositiveDurationEnvOrDefault("CLEANUP_INTERVAL_HOURS", 24, time.Hour)
	if err != nil {
		return nil, err
	}
	staleJobThresholdHours, err := getPositiveDurationEnvOrDefault("STALE_JOB_THRESHOLD_HOURS", 24, time.Hour)
	if err != nil {
		return nil, err
	}

	vars := Vars{
		WebhookSecret:          os.Getenv("WEBHOOK_SECRET"),
		Port:                   getEnvOrDefault("PORT", "8080"),
		DatabasePath:           getEnvOrDefault("DATABASE_PATH", "./data/live-actions.db"),
		LogLevel:               getEnvOrDefault("LOG_LEVEL", "info"),
		TLSEnabled:             getEnvOrDefault("TLS_ENABLED", "false") == "true",
		Environment:            getEnvOrDefault("ENVIRONMENT", "development"),
		DataRetentionDays:      dataRetentionDays,       // Default 1 month
		CleanupIntervalHours:   cleanupIntervalHours,    // Daily cleanup
		StaleJobThresholdHours: staleJobThresholdHours, // Jobs queued/in_progress longer than this are considered stale

		WebhookTransport: getEnvOrDefault("WEBHOOK_TRANSPORT", "http"),
		GitHubToken:      os.Getenv("GITHUB_TOKEN"),
		GitHubHost:       getEnvOrDefault("GITHUB_HOST", "github.com"),
		GitHubRepo:       os.Getenv("GITHUB_REPO"),
		GitHubRepos:      os.Getenv("GITHUB_REPOS"),
		GitHubOrg:        os.Getenv("GITHUB_ORG"),
		GitHubEnterprise: os.Getenv("GITHUB_ENTERPRISE"),
		GitHubEvents:     getEnvOrDefault("GITHUB_EVENTS", "workflow_job,workflow_run"),
	}

	config := &Config{Vars: vars}

	// Validate critical configuration in production
	if config.IsProduction() {
		if vars.WebhookSecret == "" {
			return nil, fmt.Errorf("WEBHOOK_SECRET is required in production")
		}
	}

	switch vars.WebhookTransport {
	case "http", "websocket":
	default:
		return nil, fmt.Errorf("WEBHOOK_TRANSPORT must be \"http\" or \"websocket\", got %q", vars.WebhookTransport)
	}

	if vars.WebhookTransport == "websocket" {
		if vars.GitHubToken == "" {
			return nil, fmt.Errorf("GITHUB_TOKEN is required when WEBHOOK_TRANSPORT=websocket")
		}
		set := 0
		if vars.GitHubRepo != "" {
			set++
		}
		if vars.GitHubRepos != "" {
			set++
			seenRepos := make(map[string]struct{})
			for _, repo := range strings.Split(vars.GitHubRepos, ",") {
				repo = strings.TrimSpace(repo)
				parts := strings.Split(repo, "/")
				if len(parts) != 2 || parts[0] == "" || parts[0] != strings.TrimSpace(parts[0]) || parts[1] == "" || parts[1] != strings.TrimSpace(parts[1]) {
					return nil, fmt.Errorf("GITHUB_REPOS must be a comma-separated list of owner/repo values")
				}
				repoKey := strings.ToLower(repo)
				if _, exists := seenRepos[repoKey]; exists {
					return nil, fmt.Errorf("GITHUB_REPOS must not contain duplicate repositories")
				}
				seenRepos[repoKey] = struct{}{}
			}
		}
		if vars.GitHubOrg != "" {
			set++
		}
		if vars.GitHubEnterprise != "" {
			set++
		}
		if set == 0 {
			return nil, fmt.Errorf("one of GITHUB_REPO, GITHUB_REPOS, GITHUB_ORG, or GITHUB_ENTERPRISE is required when WEBHOOK_TRANSPORT=websocket")
		}
		if set > 1 {
			return nil, fmt.Errorf("set only one of GITHUB_REPO, GITHUB_REPOS, GITHUB_ORG, and GITHUB_ENTERPRISE")
		}
	}

	return config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getPositiveDurationEnvOrDefault parses a positive integer duration and checks
// that converting it to time.Duration cannot overflow. An unset variable uses
// the supplied default.
func getPositiveDurationEnvOrDefault(key string, defaultValue int, unit time.Duration) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, raw)
	}

	maxValue := int64((1<<63 - 1) / int64(unit))
	if int64(value) > maxValue {
		return 0, fmt.Errorf("%s exceeds the maximum duration value of %d", key, maxValue)
	}
	return value, nil
}

func (c *Config) GetDatabasePath() string {
	return c.Vars.DatabasePath
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Vars.Environment == "production"
}

// IsHTTPS returns true if TLS is enabled
func (c *Config) IsHTTPS() bool {
	return c.Vars.TLSEnabled
}

// GetDataRetentionDuration returns the data retention period as a time.Duration
func (c *Config) GetDataRetentionDuration() time.Duration {
	return time.Duration(c.Vars.DataRetentionDays) * 24 * time.Hour
}

// GetCleanupInterval returns the cleanup interval as a time.Duration
func (c *Config) GetCleanupInterval() time.Duration {
	return time.Duration(c.Vars.CleanupIntervalHours) * time.Hour
}

// GetStaleJobThreshold returns the stale job threshold as a time.Duration
func (c *Config) GetStaleJobThreshold() time.Duration {
	return time.Duration(c.Vars.StaleJobThresholdHours) * time.Hour
}
