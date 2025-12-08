package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// WebhookReceivedTotal is the total number of webhooks received
	WebhookReceivedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "webhook",
			Name:      "received_total",
			Help:      "Total number of webhooks received from GitHub",
		},
		[]string{"event_type", "status", "runs_on"},
	)

	// WebhookProcessingDuration is the duration of webhook processing
	WebhookProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "webhook",
			Name:      "processing_duration_seconds",
			Help:      "Duration of webhook processing in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"event_type", "runs_on"},
	)

	// WebhookJobsEnqueued is the total number of jobs enqueued from webhooks
	WebhookJobsEnqueued = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "webhook",
			Name:      "jobs_enqueued_total",
			Help:      "Total number of jobs enqueued from webhooks",
		},
		[]string{"event_type", "repository", "runs_on"},
	)
)
