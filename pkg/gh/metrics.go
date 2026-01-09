package gh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/m4ns0ur/httpcache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const githubAPINamespace = "myshoes"

var (
	githubAPIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: githubAPINamespace,
			Subsystem: "github_api",
			Name:      "requests_total",
			Help:      "Total number of GitHub API requests.",
		},
		[]string{"path", "method", "status_class"},
	)
	githubAPIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: githubAPINamespace,
			Subsystem: "github_api",
			Name:      "request_duration_seconds",
			Help:      "Duration of GitHub API requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"path", "method", "status_class"},
	)
	githubAPIErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: githubAPINamespace,
			Subsystem: "github_api",
			Name:      "errors_total",
			Help:      "Total number of GitHub API request errors.",
		},
		[]string{"path", "method", "error_type"},
	)
	githubAPIInflight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: githubAPINamespace,
			Subsystem: "github_api",
			Name:      "inflight",
			Help:      "Number of in-flight GitHub API requests.",
		},
		[]string{"path", "method"},
	)
	githubAPICacheTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: githubAPINamespace,
			Subsystem: "github_api",
			Name:      "cache_total",
			Help:      "Total number of GitHub API cache hits/misses.",
		},
		[]string{"path", "method", "result"},
	)
)

type instrumentedTransport struct {
	next http.RoundTripper
}

func newInstrumentedTransport(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	if _, ok := next.(*instrumentedTransport); ok {
		return next
	}
	return &instrumentedTransport{next: next}
}

func (t *instrumentedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	path := "unknown"
	method := "UNKNOWN"
	if req != nil {
		method = req.Method
		if req.URL != nil && req.URL.Path != "" {
			path = req.URL.Path
		}
	}

	githubAPIInflight.WithLabelValues(path, method).Inc()
	defer githubAPIInflight.WithLabelValues(path, method).Dec()

	resp, err := t.next.RoundTrip(req)

	statusClass := "error"
	if err == nil && resp != nil {
		statusClass = fmt.Sprintf("%dxx", resp.StatusCode/100)
	}
	githubAPIRequestsTotal.WithLabelValues(path, method, statusClass).Inc()
	githubAPIRequestDuration.WithLabelValues(path, method, statusClass).Observe(time.Since(start).Seconds())

	if err != nil {
		githubAPIErrorsTotal.WithLabelValues(path, method, classifyGitHubAPIError(err)).Inc()
		return resp, err
	}

	if resp != nil {
		cacheResult := "miss"
		if resp.Header.Get(httpcache.XFromCache) == "1" {
			cacheResult = "hit"
		}
		githubAPICacheTotal.WithLabelValues(path, method, cacheResult).Inc()
	}

	return resp, err
}

func classifyGitHubAPIError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	return "transport"
}
