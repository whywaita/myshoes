package starter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// AddInstanceBackoffDuration is histogram of exponential backoff duration for adding instance
	AddInstanceBackoffDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "myshoes",
		Subsystem: "starter",
		Name:      "add_instance_backoff_duration_seconds",
		Help:      "Histogram of exponential backoff duration in seconds for adding instance",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s, 2s, 4s, 8s, 16s, 32s, 64s, 128s, 256s, 512s
	}, []string{"job_uuid"})

	// AddInstanceRetryTotal is counter of total retries for adding instance
	AddInstanceRetryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "myshoes",
		Subsystem: "starter",
		Name:      "add_instance_retry_total",
		Help:      "Total number of retries for adding instance",
	}, []string{"job_uuid"})
)
