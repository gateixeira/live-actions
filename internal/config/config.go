package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Vars struct {
	WebhookSecret        string
	Port                 string
	DatabaseURL          string
	LogLevel             string
	TLSEnabled           bool
	Environment          string
	Labels               []string
	DataRetentionDays    int
	CleanupIntervalHours int
	PrometheusURL        string
	RunnerTypeConfigPath string
}

type Config struct {
	Vars             Vars
	RunnerTypeConfig *RunnerTypeConfig
}

// NewAppState creates and initializes a new application state
func NewConfig() *Config {
	vars := Vars{
		WebhookSecret:        os.Getenv("WEBHOOK_SECRET"),
		Port:                 getEnvOrDefault("PORT", "8080"),
		DatabaseURL:          getEnvOrDefault("DATABASE_URL", "postgresql://postgres:@localhost:5432/live-actions?sslmode=disable"),
		LogLevel:             getEnvOrDefault("LOG_LEVEL", "info"),
		TLSEnabled:           getEnvOrDefault("TLS_ENABLED", "false") == "true",
		Environment:          getEnvOrDefault("ENVIRONMENT", "development"),
		DataRetentionDays:    getEnvOrDefaultInt("DATA_RETENTION_DAYS", 30),    // Default 1 month
		CleanupIntervalHours: getEnvOrDefaultInt("CLEANUP_INTERVAL_HOURS", 24), // Daily cleanup
		PrometheusURL:        getEnvOrDefault("PROMETHEUS_URL", "http://localhost:9090"),
		RunnerTypeConfigPath: getEnvOrDefault("RUNNER_TYPE_CONFIG_PATH", "config/runner_types.json"),
	}

	config := &Config{Vars: vars}

	runnerTypeConfig, err := LoadRunnerTypeConfig(vars.RunnerTypeConfigPath)
	if err != nil {
		panic(fmt.Errorf("failed to load runner type config: %w", err))
	}
	config.RunnerTypeConfig = runnerTypeConfig

	// Validate critical configuration in production
	if config.IsProduction() {
		if vars.WebhookSecret == "" {
			panic("WEBHOOK_SECRET is required in production")
		}
		if vars.DatabaseURL == "" {
			panic("DATABASE_URL is required in production")
		}
	}

	return config
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvOrDefaultInt gets an environment variable as an integer or returns the default value
func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func (c *Config) GetDSN() string {
	return c.Vars.DatabaseURL
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
