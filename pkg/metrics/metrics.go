package metrics

import (
	"github.com/gateixeira/live-actions/models"
	"github.com/prometheus/client_golang/prometheus"
)

// Registry holds all Prometheus metrics
type Registry struct {
	// Current state metrics (gauges)
	CurrentJobs *prometheus.GaugeVec

	// Historical metrics
	QueueDurationSeconds *prometheus.HistogramVec
}

// NewRegistry creates and registers all Prometheus metrics
func NewRegistry() *Registry {
	r := &Registry{
		// Current state gauges
		CurrentJobs: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "github_runners_jobs",
			Help: "Current number of jobs by status and runner type",
		}, []string{"runner_type", "job_status"}),

		// Historical counters and histograms
		QueueDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "github_runners_queue_duration_seconds",
				Help:    "Time spent waiting in queue before job execution starts",
				Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1200, 1800, 3600}, // 1s to 1h
			},
			[]string{"runner_type"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		r.CurrentJobs,
		r.QueueDurationSeconds,
	)

	return r
}

func (r *Registry) RecordQueueDuration(runnerType models.RunnerType, durationSeconds float64) {
	r.QueueDurationSeconds.WithLabelValues(string(runnerType)).Observe(durationSeconds)
}

func (r *Registry) UpdateCurrentJobCounts(jobCounts map[string]map[string]int) {
	for runnerType, statusCounts := range jobCounts {
		for status, count := range statusCounts {
			r.CurrentJobs.WithLabelValues(runnerType, status).Set(float64(count))
		}
	}
}
