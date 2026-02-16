CREATE TABLE IF NOT EXISTS workflow_runs (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    repository TEXT,
    html_url TEXT,
    display_title TEXT,
    conclusion TEXT,
    created_at TEXT NOT NULL,
    run_started_at TEXT,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS workflow_jobs (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    run_id INTEGER NOT NULL,
    status TEXT NOT NULL,
    runner_type TEXT NOT NULL,
    labels TEXT,
    conclusion TEXT,
    created_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS webhook_events (
    delivery_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    sequence_id INTEGER NOT NULL,
    github_timestamp TEXT NOT NULL,
    received_at TEXT NOT NULL DEFAULT (datetime('now')),
    processed_at TEXT,
    raw_payload TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    ordering_key TEXT NOT NULL,
    status_priority INTEGER NOT NULL
);

-- Workflow Jobs
CREATE INDEX IF NOT EXISTS idx_workflow_jobs_workflow_run_id ON workflow_jobs (run_id);
CREATE INDEX IF NOT EXISTS idx_workflow_jobs_created_at ON workflow_jobs (created_at);
CREATE INDEX IF NOT EXISTS idx_workflow_jobs_status_runner_type ON workflow_jobs (status, runner_type);
CREATE INDEX IF NOT EXISTS idx_workflow_jobs_labels_runner_type ON workflow_jobs (runner_type, labels);

-- Workflow Runs
CREATE INDEX IF NOT EXISTS idx_workflow_runs_created_at ON workflow_runs (created_at);

-- Webhook Events
CREATE INDEX IF NOT EXISTS idx_webhook_events_status_ordering ON webhook_events (status, github_timestamp, ordering_key, status_priority);
CREATE INDEX IF NOT EXISTS idx_webhook_events_status_received_at ON webhook_events (status, received_at);
CREATE INDEX IF NOT EXISTS idx_webhook_events_processed_at ON webhook_events (processed_at);