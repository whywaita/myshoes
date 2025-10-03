package runner

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// DeleteRunnerBackoffDuration is histogram of exponential backoff duration for deleting runner
	DeleteRunnerBackoffDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "myshoes",
		Subsystem: "runner",
		Name:      "delete_runner_backoff_duration_seconds",
		Help:      "Histogram of exponential backoff duration in seconds for deleting runner",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s, 2s, 4s, 8s, 16s, 32s, 64s, 128s, 256s, 512s
	})

	// DeleteRunnerRetryTotal is counter of total retries for deleting runner
	DeleteRunnerRetryTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "myshoes",
		Subsystem: "runner",
		Name:      "delete_runner_retry_total",
		Help:      "Total number of retries for deleting runner",
	})
)
