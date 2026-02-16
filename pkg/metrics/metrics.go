package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Registry holds all Prometheus metrics
type Registry struct {
	// Current state metrics (gauges)
	CurrentJobs *prometheus.GaugeVec

	// Historical metrics
	QueueDurationSeconds prometheus.Histogram
}

// NewRegistry creates and registers all Prometheus metrics
func NewRegistry() *Registry {
	r := &Registry{
		// Current state gauges
		CurrentJobs: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "github_runners_jobs",
			Help: "Current number of jobs by status",
		}, []string{"job_status"}),

		// Historical counters and histograms
		QueueDurationSeconds: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "github_runners_queue_duration_seconds",
				Help:    "Time spent waiting in queue before job execution starts",
				Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1200, 1800, 3600}, // 1s to 1h
			},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		r.CurrentJobs,
		r.QueueDurationSeconds,
	)

	return r
}

func (r *Registry) RecordQueueDuration(durationSeconds float64) {
	r.QueueDurationSeconds.Observe(durationSeconds)
}

func (r *Registry) UpdateCurrentJobCounts(running, queued int) {
	r.CurrentJobs.WithLabelValues("in_progress").Set(float64(running))
	r.CurrentJobs.WithLabelValues("queued").Set(float64(queued))
}
