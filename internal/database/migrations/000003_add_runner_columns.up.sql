-- Runner that picked up the job. GitHub sends null while the job is queued,
-- so both columns stay NULL (or empty) until a runner is assigned.
ALTER TABLE workflow_jobs ADD COLUMN runner_name TEXT;
ALTER TABLE workflow_jobs ADD COLUMN runner_group_name TEXT;

-- Runner stats: per-runner aggregation filtered on completed_at >= ?
CREATE INDEX IF NOT EXISTS idx_workflow_jobs_runner_name_completed_at ON workflow_jobs (runner_name, completed_at);
