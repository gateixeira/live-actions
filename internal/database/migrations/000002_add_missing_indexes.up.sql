-- Repository filtering: paginated queries, DISTINCT autocomplete, and JOIN-based analytics
CREATE INDEX IF NOT EXISTS idx_workflow_runs_repository ON workflow_runs (repository);

-- Failure analytics: queries filter on status = 'completed' AND completed_at >= ?
CREATE INDEX IF NOT EXISTS idx_workflow_jobs_status_completed_at ON workflow_jobs (status, completed_at);

-- Avg queue time: GetMetricsSummary filters on started_at >= ?
CREATE INDEX IF NOT EXISTS idx_workflow_jobs_started_at ON workflow_jobs (started_at);
