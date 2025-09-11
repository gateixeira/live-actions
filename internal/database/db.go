package database

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/gateixeira/live-actions/pkg/logger"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

var DB *sql.DB

// InitDB initializes the PostgreSQL database connection and runs migrations
func InitDB(dsn string) error {
	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	// Set connection pool parameters
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)

	migrationsPath := filepath.Join(".", "migrations")

	if err = RunMigrations(DB, migrationsPath); err != nil {
		logger.Logger.Error("Failed to run database migrations", zap.Error(err))
		return err
	}

	logger.Logger.Info("Database initialized successfully")
	return nil
}

// CloseDB closes the database connection
func CloseDB() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

// RunMigrations performs all necessary database migrations
func RunMigrations(db *sql.DB, migrationsPath string) error {
	logger.Logger.Info("Running database migrations...")

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create database driver: %v", err)
	}

	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return fmt.Errorf("could not get absolute path: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", absPath),
		"postgres",
		driver,
	)
	if err != nil {
		logger.Logger.Error("could not create migration instance", zap.Error(err))
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Logger.Error("could not run migrations", zap.Error(err))
		return err
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		logger.Logger.Error("could not get migration version", zap.Error(err))
		return err
	}

	logger.Logger.Info("Database migrations completed",
		zap.Uint("version", version),
		zap.Bool("dirty", dirty))

	return nil
}
