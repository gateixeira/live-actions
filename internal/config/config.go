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
	DatabasePath         string
	LogLevel             string
	TLSEnabled           bool
	Environment          string
	DataRetentionDays    int
	CleanupIntervalHours int
}

type Config struct {
	Vars Vars
}

// NewConfig creates and initializes a new application config.
func NewConfig() (*Config, error) {
	vars := Vars{
		WebhookSecret:        os.Getenv("WEBHOOK_SECRET"),
		Port:                 getEnvOrDefault("PORT", "8080"),
		DatabasePath:         getEnvOrDefault("DATABASE_PATH", "./data/live-actions.db"),
		LogLevel:             getEnvOrDefault("LOG_LEVEL", "info"),
		TLSEnabled:           getEnvOrDefault("TLS_ENABLED", "false") == "true",
		Environment:          getEnvOrDefault("ENVIRONMENT", "development"),
		DataRetentionDays:    getEnvOrDefaultInt("DATA_RETENTION_DAYS", 30),    // Default 1 month
		CleanupIntervalHours: getEnvOrDefaultInt("CLEANUP_INTERVAL_HOURS", 24), // Daily cleanup
	}

	config := &Config{Vars: vars}

	// Validate critical configuration in production
	if config.IsProduction() {
		if vars.WebhookSecret == "" {
			return nil, fmt.Errorf("WEBHOOK_SECRET is required in production")
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

// getEnvOrDefaultInt gets an environment variable as an integer or returns the default value
func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
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
