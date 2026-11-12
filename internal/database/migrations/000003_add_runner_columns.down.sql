DROP INDEX IF EXISTS idx_workflow_jobs_runner_name_completed_at;
ALTER TABLE workflow_jobs DROP COLUMN runner_group_name;
ALTER TABLE workflow_jobs DROP COLUMN runner_name;
