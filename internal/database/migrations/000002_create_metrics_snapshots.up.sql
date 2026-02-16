CREATE TABLE IF NOT EXISTS metrics_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    running_jobs INTEGER NOT NULL DEFAULT 0,
    queued_jobs INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_metrics_snapshots_timestamp ON metrics_snapshots (timestamp);
