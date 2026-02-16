DROP INDEX IF EXISTS idx_webhook_events_status_ordering;
DROP INDEX IF EXISTS idx_webhook_events_status_received_at;
DROP INDEX IF EXISTS idx_webhook_events_processed_at;

-- Drop workflow indices
DROP INDEX IF EXISTS idx_workflow_jobs_workflow_run_id;
DROP INDEX IF EXISTS idx_workflow_jobs_created_at;
DROP INDEX IF EXISTS idx_workflow_jobs_status_runner_type;
DROP INDEX IF EXISTS idx_workflow_jobs_labels_runner_type;
DROP INDEX IF EXISTS idx_workflow_runs_created_at;

-- Drop tables
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS workflow_jobs;
DROP TABLE IF EXISTS workflow_runs;