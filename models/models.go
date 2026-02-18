package models

import (
	"time"
)

// JobStatus represents the status of a workflow job
type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusInProgress JobStatus = "in_progress"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusWaiting    JobStatus = "waiting"
	JobStatusRequested  JobStatus = "requested"
	JobStatusCancelled  JobStatus = "cancelled"
)

// WebhookEvent represents the incoming webhook payload
type WorkflowJobEvent struct {
	Action      string      `json:"action" binding:"required"`
	WorkflowJob WorkflowJob `json:"workflow_job" binding:"required"`
}

type WorkflowRunEvent struct {
	Action      string      `json:"action" binding:"required"`
	Repository  Repository  `json:"repository" binding:"required"`
	WorkflowRun WorkflowRun `json:"workflow_run" binding:"required"`
}

type WorkflowJob struct {
	ID          int64     `json:"id" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Status      JobStatus `json:"status" binding:"required"`
	Labels      []string  `json:"labels" binding:"required"`
	HtmlUrl     string    `json:"html_url"`
	Conclusion  string    `json:"conclusion"`
	CreatedAt   time.Time `json:"created_at" binding:"required"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	RunID       int64     `json:"run_id" binding:"required"`
}

type WorkflowRun struct {
	ID             int64     `json:"id" binding:"required"`
	Name           string    `json:"name" binding:"required"`
	Status         JobStatus `json:"status" binding:"required"`
	HtmlUrl        string    `json:"html_url" binding:"required"`
	DisplayTitle   string    `json:"display_title" binding:"required"`
	Conclusion     string    `json:"conclusion"`
	CreatedAt      time.Time `json:"created_at" binding:"required"`
	RunStartedAt   time.Time `json:"run_started_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	RepositoryName string    `json:"repository_name"`
}

type Repository struct {
	Name string `json:"name" binding:"required"`
	Url  string `json:"url" binding:"required"`
}

type MetricsUpdateEvent struct {
	RunningJobs int    `json:"running_jobs"`
	QueuedJobs  int    `json:"queued_jobs"`
	Timestamp   string `json:"timestamp"`
}

type WorkflowUpdateEvent struct {
	Type        string      `json:"type"` // "run" or "job"
	Action      string      `json:"action"`
	ID          int64       `json:"id"`
	Status      string      `json:"status"`
	Timestamp   string      `json:"timestamp"`
	WorkflowJob WorkflowJob `json:"workflow_job,omitempty"`
	WorkflowRun WorkflowRun `json:"workflow_run,omitempty"`
}

type EventSequence struct {
	EventID    string    `json:"event_id"`
	SequenceID int64     `json:"sequence_id"`
	Timestamp  time.Time `json:"timestamp"`
	DeliveryID string    `json:"delivery_id"`
	ReceivedAt time.Time `json:"received_at"`
}

type OrderedEvent struct {
	Sequence       EventSequence `json:"sequence"`
	EventType      string        `json:"event_type"`
	RawPayload     []byte        `json:"raw_payload"`
	ProcessedAt    *time.Time    `json:"processed_at,omitempty"`
	OrderingKey    string        `json:"ordering_key"`
	StatusPriority int           `json:"status_priority"`
}

type EventBuffer struct {
	Events    map[string]*OrderedEvent
	Queue     []*OrderedEvent
	MaxBuffer int
	MaxAge    time.Duration
}

type MetricsResponse struct {
	CurrentMetrics map[string]float64 `json:"current_metrics"`
	TimeSeries     struct {
		RunningJobs TimeSeriesData `json:"running_jobs"`
		QueuedJobs  TimeSeriesData `json:"queued_jobs"`
	} `json:"time_series"`
}

// TimeSeriesData represents time series data for charts
type TimeSeriesData struct {
	Status string              `json:"status"`
	Data   TimeSeriesDataInner `json:"data"`
}

// TimeSeriesDataInner contains the actual time series results
type TimeSeriesDataInner struct {
	ResultType string            `json:"resultType"`
	Result     []TimeSeriesEntry `json:"result"`
}

// TimeSeriesEntry represents a single time series entry
type TimeSeriesEntry struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

// MetricsSnapshot is a point-in-time record of job counts stored in the DB.
type MetricsSnapshot struct {
	Timestamp int64 `json:"timestamp"`
	Running   int   `json:"running"`
	Queued    int   `json:"queued"`
}

// FailingJob represents a job's failure statistics.
type FailingJob struct {
	Name        string  `json:"name"`
	HtmlUrl     string  `json:"html_url"`
	Failures    int     `json:"failures"`
	Total       int     `json:"total"`
	FailureRate float64 `json:"failure_rate"`
}

// FailureAnalytics contains summary failure metrics.
type FailureAnalytics struct {
	TotalCompleted  int          `json:"total_completed"`
	TotalFailed     int          `json:"total_failed"`
	TotalCancelled  int          `json:"total_cancelled"`
	FailureRate     float64      `json:"failure_rate"`
	TopFailingJobs  []FailingJob `json:"top_failing_jobs"`
}

// FailureTrendPoint represents failure counts at a point in time.
type FailureTrendPoint struct {
	Timestamp  int64 `json:"timestamp"`
	Failures   int   `json:"failures"`
	Successes  int   `json:"successes"`
	Cancelled  int   `json:"cancelled"`
}

// LabelDemandSummary represents aggregate demand stats for a single runner label.
type LabelDemandSummary struct {
	Label           string  `json:"label"`
	TotalJobs       int     `json:"total_jobs"`
	Running         int     `json:"running"`
	Queued          int     `json:"queued"`
	AvgQueueSeconds float64 `json:"avg_queue_seconds"`
}

// LabelDemandTrendPoint represents demand for a single label at a point in time.
type LabelDemandTrendPoint struct {
	Timestamp int64  `json:"timestamp"`
	Label     string `json:"label"`
	Running   int    `json:"running"`
	Queued    int    `json:"queued"`
}
