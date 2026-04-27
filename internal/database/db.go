package database

import (
	"database/sql"
	"embed"
	"fmt"
	"net/url"
	"path"
	"runtime"
	"sort"
	"strings"

	"github.com/gateixeira/live-actions/pkg/logger"
	_ "modernc.org/sqlite"
	"go.uber.org/zap"
)

//go:embed migrations/*.up.sql
var migrationsFS embed.FS

// connPragmas are applied to every new connection in both the write and read
// pools via the DSN. Pragmas in SQLite are connection-scoped (except
// journal_mode, which is persistent), so encoding them in the DSN guarantees
// they are set on every pooled connection rather than only the first one.
var connPragmas = []string{
	"journal_mode(WAL)",
	"busy_timeout(5000)",
	"synchronous(NORMAL)",
	"foreign_keys(ON)",
}

// InitDB opens two *sql.DB handles against the same SQLite file:
//   - writeDB: capped at 1 open connection because SQLite supports a single
//     writer at a time. All INSERT/UPDATE/DELETE/BEGIN traffic goes here.
//   - readDB: pooled for concurrent SELECTs. With WAL, readers do not block
//     the writer and vice versa, so a slow analytics query no longer stalls
//     webhook ingestion.
//
// Migrations are run on the writer.
func InitDB(dsn string) (writeDB, readDB *sql.DB, err error) {
	connDSN := buildDSN(dsn, connPragmas)

	writeDB, err = sql.Open("sqlite", connDSN)
	if err != nil {
		return nil, nil, err
	}
	if err = writeDB.Ping(); err != nil {
		_ = writeDB.Close()
		return nil, nil, err
	}
	writeDB.SetMaxOpenConns(1)

	readDB, err = sql.Open("sqlite", connDSN)
	if err != nil {
		_ = writeDB.Close()
		return nil, nil, err
	}
	if err = readDB.Ping(); err != nil {
		_ = writeDB.Close()
		_ = readDB.Close()
		return nil, nil, err
	}
	readPoolSize := runtime.NumCPU()
	if readPoolSize < 4 {
		readPoolSize = 4
	}
	readDB.SetMaxOpenConns(readPoolSize)

	if err = RunMigrations(writeDB); err != nil {
		logger.Logger.Error("Failed to run database migrations", zap.Error(err))
		_ = writeDB.Close()
		_ = readDB.Close()
		return nil, nil, err
	}

	logger.Logger.Info("Database initialized successfully",
		zap.Int("read_pool_size", readPoolSize),
	)
	return writeDB, readDB, nil
}

// buildDSN appends _pragma query parameters to the given dsn so the
// modernc.org/sqlite driver applies them to every new connection in the pool.
func buildDSN(dsn string, pragmas []string) string {
	if len(pragmas) == 0 {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	parts := make([]string, 0, len(pragmas))
	for _, p := range pragmas {
		parts = append(parts, "_pragma="+url.QueryEscape(p))
	}
	return dsn + sep + strings.Join(parts, "&")
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
			_ = tx.Rollback()
			return fmt.Errorf("failed to apply migration %s: %w", m.file, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version); err != nil {
			_ = tx.Rollback()
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
