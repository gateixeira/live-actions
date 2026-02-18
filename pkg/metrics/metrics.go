package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
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
	}

	prometheus.MustRegister(
		r.CurrentJobs,
		r.JobsByLabel,
		r.QueueDurationSeconds,
		r.JobConclusionsTotal,
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
