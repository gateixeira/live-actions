export type JobStatus =
  | 'queued'
  | 'in_progress'
  | 'completed'
  | 'waiting'
  | 'requested'
  | 'cancelled'

export interface WorkflowRun {
  id: number
  name: string
  status: JobStatus
  html_url: string
  display_title: string
  conclusion: string
  created_at: string
  run_started_at: string
  updated_at: string
  repository_name: string
}

export interface WorkflowJob {
  id: number
  name: string
  status: JobStatus
  labels: string[]
  html_url: string
  conclusion: string
  created_at: string
  started_at: string
  completed_at: string
  run_id: number
}

export interface Pagination {
  current_page: number
  total_pages: number
  total_count: number
  page_size: number
  has_next: boolean
  has_previous: boolean
}

export interface WorkflowRunsResponse {
  workflow_runs: WorkflowRun[]
  pagination: Pagination
}

export interface WorkflowJobsResponse {
  workflow_jobs: WorkflowJob[]
}

export interface MetricsUpdateEvent {
  running_jobs: number
  queued_jobs: number
  timestamp: string
}

export interface WorkflowUpdateEvent {
  type: 'run' | 'job'
  action: string
  id: number
  status: string
  timestamp: string
  workflow_job?: WorkflowJob
  workflow_run?: WorkflowRun
}

export interface TimeSeriesEntry {
  metric: Record<string, string>
  values: [number, string][]
}

export interface TimeSeriesData {
  status: string
  data: {
    resultType: string
    result: TimeSeriesEntry[]
  }
}

export interface MetricsResponse {
  current_metrics: Record<string, number>
  time_series: {
    running_jobs: TimeSeriesData
    queued_jobs: TimeSeriesData
  }
}

export type Period = 'hour' | 'day' | 'week' | 'month'

export interface FailingJob {
  name: string
  html_url: string
  failures: number
  total: number
  failure_rate: number
}

export interface FailureAnalyticsSummary {
  total_completed: number
  total_failed: number
  total_cancelled: number
  failure_rate: number
  top_failing_jobs: FailingJob[]
}

export interface FailureTrendPoint {
  timestamp: number
  failures: number
  successes: number
  cancelled: number
}

export interface FailureAnalyticsResponse {
  summary: FailureAnalyticsSummary
  trend: FailureTrendPoint[]
}

export interface LabelDemandSummary {
  label: string
  total_jobs: number
  running: number
  queued: number
  avg_queue_seconds: number
}

export interface LabelDemandTrendPoint {
  timestamp: number
  label: string
  count: number
}

export interface LabelDemandResponse {
  summary: LabelDemandSummary[]
  trend: LabelDemandTrendPoint[]
}

export interface RepositoriesResponse {
  repositories: string[]
}
