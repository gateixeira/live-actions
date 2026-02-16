package database

import (
	"database/sql"
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/gateixeira/live-actions/pkg/logger"
	_ "modernc.org/sqlite"
	"go.uber.org/zap"
)

//go:embed migrations/*.up.sql
var migrationsFS embed.FS

// InitDB initializes the SQLite database connection and runs migrations
func InitDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	// SQLite pragmas for performance and reliability
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("failed to set pragma %s: %w", p, err)
		}
	}

	// SQLite handles concurrency at the file level; keep pool small
	db.SetMaxOpenConns(1)

	if err = RunMigrations(db); err != nil {
		logger.Logger.Error("Failed to run database migrations", zap.Error(err))
		return nil, err
	}

	logger.Logger.Info("Database initialized successfully")
	return db, nil
}

// RunMigrations applies pending SQL migration files from the embedded migrations/ directory.
func RunMigrations(db *sql.DB) error {
	logger.Logger.Info("Running database migrations...")

	// Create migrations tracking table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Check current version
	var currentVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to check migration version: %w", err)
	}

	// Discover migration files sorted by version number
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read embedded migrations: %w", err)
	}

	type migration struct {
		version int
		file    string
	}
	var migrations []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		var ver int
		if _, err := fmt.Sscanf(e.Name(), "%06d_", &ver); err != nil {
			continue
		}
		migrations = append(migrations, migration{version: ver, file: e.Name()})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })

	// Apply pending migrations
	applied := 0
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}

		data, err := migrationsFS.ReadFile(path.Join("migrations", m.file))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", m.file, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start migration transaction: %w", err)
		}

		if _, err := tx.Exec(string(data)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %s: %w", m.file, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration version %d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", m.file, err)
		}

		logger.Logger.Info("Applied migration", zap.String("file", m.file), zap.Int("version", m.version))
		applied++
	}

	if applied == 0 {
		logger.Logger.Info("Database migrations up to date", zap.Int("version", currentVersion))
	} else {
		logger.Logger.Info("Database migrations completed", zap.Int("applied", applied))
	}

	return nil
}
