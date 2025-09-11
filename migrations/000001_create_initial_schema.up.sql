CREATE TABLE IF NOT EXISTS workflow_runs (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    repository TEXT,
    html_url TEXT,
    display_title TEXT,
    conclusion TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    run_started_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workflow_jobs (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    run_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    runner_type TEXT NOT NULL,
    labels TEXT[],
    conclusion TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS webhook_events (
    delivery_id VARCHAR(255) PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    sequence_id BIGINT NOT NULL,
    github_timestamp TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    raw_payload JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, processed, failed
    ordering_key VARCHAR(100) NOT NULL,
    status_priority INTEGER NOT NULL
);

-- Workflow Jobs
CREATE INDEX idx_workflow_jobs_workflow_run_id ON workflow_jobs (run_id);
CREATE INDEX idx_workflow_jobs_created_at ON workflow_jobs (created_at);
CREATE INDEX idx_workflow_jobs_status_runner_type ON workflow_jobs (status, runner_type);
CREATE INDEX idx_workflow_jobs_labels_runner_type ON workflow_jobs (runner_type, labels);

-- Workflow Runs  
CREATE INDEX idx_workflow_runs_created_at ON workflow_runs (created_at);

-- Webhook Events
CREATE INDEX idx_webhook_events_status_ordering ON webhook_events (status, github_timestamp, ordering_key, status_priority);
CREATE INDEX idx_webhook_events_status_received_at ON webhook_events (status, received_at);
CREATE INDEX idx_webhook_events_processed_at ON webhook_events (processed_at);