package metrics

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry holds all Prometheus metrics
type Registry struct {
	// Current state metrics (gauges)
	CurrentJobs *prometheus.GaugeVec

	// Per-label current state (gauges)
	JobsByLabel *prometheus.GaugeVec

	// Historical metrics
	QueueDurationSeconds *prometheus.HistogramVec

	// Job completion counters
	JobConclusionsTotal *prometheus.CounterVec

	// Webhook ingest counters: outcome ∈ {accepted, rejected_queue_full,
	// rejected_invalid, ignored, spilled, spill_failed, permanent_failure}.
	WebhookEventsTotal *prometheus.CounterVec

	// Ingest queue capacity is set once at startup; depth is supplied by the
	// service via RegisterIngestQueueDepth.
	IngestQueueCapacity prometheus.Gauge

	// Flush worker observability.
	FlushBatchDurationSeconds prometheus.Histogram
	FlushBatchEvents          prometheus.Histogram

	// SSE fanout observability.
	SSEEventsBroadcastTotal *prometheus.CounterVec
	SSEEventsDroppedTotal   *prometheus.CounterVec
}

// NewRegistry creates and registers all Prometheus metrics
func NewRegistry() *Registry {
	r := &Registry{
		CurrentJobs: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "github_runners_jobs",
			Help: "Current number of jobs by status",
		}, []string{"job_status"}),

		JobsByLabel: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "github_runners_jobs_by_label",
			Help: "Current number of jobs by runner label and status",
		}, []string{"label", "job_status"}),

		QueueDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "github_runners_queue_duration_seconds",
				Help:    "Time spent waiting in queue before job execution starts",
				Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1200, 1800, 3600},
			},
			[]string{"label"},
		),

		JobConclusionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "github_runners_job_conclusions_total",
			Help: "Total number of completed jobs by conclusion",
		}, []string{"conclusion"}),

		WebhookEventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "live_actions_webhook_events_total",
			Help: "Total number of webhook deliveries received, partitioned by event type and outcome",
		}, []string{"event_type", "outcome"}),

		IngestQueueCapacity: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "live_actions_ingest_queue_capacity",
			Help: "Maximum number of events the in-memory ingest queue can buffer",
		}),

		FlushBatchDurationSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "live_actions_flush_batch_duration_seconds",
			Help:    "Wall time spent draining a single flushReadyEvents tick (one or more batches)",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 12), // 1ms .. ~4s
		}),

		FlushBatchEvents: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "live_actions_flush_batch_events",
			Help:    "Number of events processed in a single flushReadyEvents tick",
			Buckets: prometheus.ExponentialBuckets(1, 2, 14), // 1 .. ~8k
		}),

		SSEEventsBroadcastTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "live_actions_sse_events_broadcast_total",
			Help: "Total number of SSE events fanned out to subscribers (counts the broadcast, not per-subscriber sends)",
		}, []string{"type"}),

		SSEEventsDroppedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "live_actions_sse_events_dropped_total",
			Help: "Total number of SSE events dropped because a subscriber's per-connection buffer was full",
		}, []string{"type"}),
	}

	prometheus.MustRegister(
		r.CurrentJobs,
		r.JobsByLabel,
		r.QueueDurationSeconds,
		r.JobConclusionsTotal,
		r.WebhookEventsTotal,
		r.IngestQueueCapacity,
		r.FlushBatchDurationSeconds,
		r.FlushBatchEvents,
		r.SSEEventsBroadcastTotal,
		r.SSEEventsDroppedTotal,
	)

	return r
}

func (r *Registry) RecordQueueDuration(label string, durationSeconds float64) {
	r.QueueDurationSeconds.WithLabelValues(label).Observe(durationSeconds)
}

func (r *Registry) UpdateCurrentJobCounts(running, queued int) {
	r.CurrentJobs.WithLabelValues("in_progress").Set(float64(running))
	r.CurrentJobs.WithLabelValues("queued").Set(float64(queued))
}

func (r *Registry) UpdateJobsByLabel(label string, running, queued int) {
	r.JobsByLabel.WithLabelValues(label, "in_progress").Set(float64(running))
	r.JobsByLabel.WithLabelValues(label, "queued").Set(float64(queued))
}

func (r *Registry) RecordJobConclusion(conclusion string) {
	r.JobConclusionsTotal.WithLabelValues(conclusion).Inc()
}

// ResetJobsByLabel clears all label gauge values before re-setting them.
func (r *Registry) ResetJobsByLabel() {
	r.JobsByLabel.Reset()
}

// RegisterIngestQueueDepth registers a GaugeFunc that reports the live ingest
// queue depth by calling the supplied closure on every scrape. Safe to call
// once at startup. Errors from duplicate registration are ignored so tests
// that re-init the global registry do not panic.
func (r *Registry) RegisterIngestQueueDepth(depth func() float64) {
	g := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "live_actions_ingest_queue_depth",
		Help: "Current number of events buffered in the in-memory ingest queue",
	}, depth)
	_ = prometheus.Register(g)
}

// RegisterSSESubscribers registers a GaugeFunc that reports the number of
// connected SSE clients by calling the supplied closure on every scrape.
func (r *Registry) RegisterSSESubscribers(count func() float64) {
	g := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "live_actions_sse_subscribers",
		Help: "Current number of connected SSE clients",
	}, count)
	_ = prometheus.Register(g)
}

// RegisterDBStats wires the standard database/sql DBStats collector for the
// supplied pool, namespaced by the given pool name (e.g. "write", "read").
func (r *Registry) RegisterDBStats(name string, db *sql.DB) {
	_ = prometheus.Register(collectors.NewDBStatsCollector(db, name))
}
