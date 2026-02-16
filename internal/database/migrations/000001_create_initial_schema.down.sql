DROP INDEX IF EXISTS idx_webhook_events_status_ordering;
DROP INDEX IF EXISTS idx_webhook_events_status_received_at;
DROP INDEX IF EXISTS idx_webhook_events_processed_at;
DROP INDEX IF EXISTS idx_workflow_jobs_workflow_run_id;
DROP INDEX IF EXISTS idx_workflow_jobs_created_at;
DROP INDEX IF EXISTS idx_workflow_runs_created_at;
DROP INDEX IF EXISTS idx_metrics_snapshots_timestamp;

DROP TABLE IF EXISTS metrics_snapshots;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS workflow_jobs;
DROP TABLE IF EXISTS workflow_runs;