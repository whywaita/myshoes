package metric

import (
	"context"
	"fmt"

	uuid "github.com/satori/go.uuid"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/docker"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/runner"
	"github.com/whywaita/myshoes/pkg/starter"
)

const memoryName = "memory"

var (
	memoryStarterMaxRunning = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "starter_max_running"),
		"The number of max running in starter (Config)",
		[]string{"starter"}, nil,
	)
	memoryStarterQueueRunning = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "starter_queue_running"),
		"running queue in starter",
		[]string{"starter"}, nil,
	)
	memoryStarterQueueWaiting = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "starter_queue_waiting"),
		"waiting queue in starter",
		[]string{"starter"}, nil,
	)
	memoryStarterRecoveredRuns = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "starter_recovered_runs"),
		"recovered runs in starter",
		[]string{"starter", "target"}, nil,
	)
	memoryGitHubRateLimitRemaining = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "github_rate_limit_remaining"),
		"The number of rate limit remaining in GitHub",
		[]string{"scope"}, nil,
	)
	memoryGitHubRateLimitLimiting = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "github_rate_limit_limiting"),
		"The number of rate limit max in GitHub",
		[]string{"scope"}, nil,
	)
	memoryDockerHubRateLimitRemaining = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "dockerhub_rate_limit_remaining"),
		"The number of rate limit remaining in DockerHub",
		[]string{}, nil,
	)
	memoryDockerHubRateLimitLimiting = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "dockerhub_rate_limit_limiting"),
		"The number of rate limit max in DockerHub",
		[]string{}, nil,
	)
	memoryRunnerMaxConcurrencyDeleting = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "runner_max_concurrency_deleting"),
		"The number of max concurrency deleting in runner (Config)",
		[]string{"runner"}, nil,
	)
	memoryRunnerQueueConcurrencyDeleting = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "runner_queue_concurrency_deleting"),
		"deleting concurrency in runner",
		[]string{"runner"}, nil,
	)
	memoryRunnerDeleteRetryCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "runner_delete_retry_count"),
		"retry count of deleting in runner",
		[]string{"runner"}, nil,
	)
	memoryRunnerCreateRetryCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "runner_create_retry_count"),
		"retry count of creating in runner",
		[]string{"runner"}, nil,
	)
)

// ScraperMemory is scraper implement for memory
type ScraperMemory struct{}

// Name return name
func (ScraperMemory) Name() string {
	return memoryName
}

// Help return help
func (ScraperMemory) Help() string {
	return "Collect from memory"
}

// Scrape scrape metrics
func (ScraperMemory) Scrape(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	if err := scrapeStarterValues(ch); err != nil {
		return fmt.Errorf("failed to scrape starter values: %w", err)
	}
	if err := scrapeGitHubValues(ch); err != nil {
		return fmt.Errorf("failed to scrape GitHub values: %w", err)
	}
	if config.Config.ProvideDockerHubMetrics {
		if err := scrapeDockerValues(ch); err != nil {
			return fmt.Errorf("failed to scrape Docker values: %w", err)
		}
	}
	return nil
}

func scrapeStarterValues(ch chan<- prometheus.Metric) error {
	configMax := config.Config.MaxConnectionsToBackend

	const labelStarter = "starter"

	ch <- prometheus.MustNewConstMetric(
		memoryStarterMaxRunning, prometheus.GaugeValue, float64(configMax), labelStarter)

	countRunning := starter.CountRunning.Load()
	countWaiting := starter.CountWaiting.Load()

	ch <- prometheus.MustNewConstMetric(
		memoryStarterQueueRunning, prometheus.GaugeValue, float64(countRunning), labelStarter)
	ch <- prometheus.MustNewConstMetric(
		memoryStarterQueueWaiting, prometheus.GaugeValue, float64(countWaiting), labelStarter)

	starter.CountRecovered.Range(func(key, value interface{}) bool {
		ch <- prometheus.MustNewConstMetric(
			memoryStarterRecoveredRuns, prometheus.GaugeValue, float64(value.(int)), labelStarter, key.(string),
		)
		return true
	})

	const labelRunner = "runner"

	configRunnerDeletingMax := config.Config.MaxConcurrencyDeleting
	countRunnerDeletingNow := runner.ConcurrencyDeleting.Load()

	ch <- prometheus.MustNewConstMetric(
		memoryRunnerMaxConcurrencyDeleting, prometheus.GaugeValue, float64(configRunnerDeletingMax), labelRunner)
	ch <- prometheus.MustNewConstMetric(
		memoryRunnerQueueConcurrencyDeleting, prometheus.GaugeValue, float64(countRunnerDeletingNow), labelRunner)

	runner.DeleteRetryCount.Range(func(key, value any) bool {
		ch <- prometheus.MustNewConstMetric(
			memoryRunnerDeleteRetryCount, prometheus.GaugeValue, float64(value.(int)), key.(uuid.UUID).String())
		return true
	})

	starter.AddInstanceRetryCount.Range(func(key, value any) bool {
		ch <- prometheus.MustNewConstMetric(
			memoryRunnerCreateRetryCount, prometheus.GaugeValue, float64(value.(int)), key.(uuid.UUID).String())
		return true
	})

	return nil
}

func scrapeGitHubValues(ch chan<- prometheus.Metric) error {
	rateLimitRemain := gh.GetRateLimitRemain()
	for scope, remain := range rateLimitRemain {
		ch <- prometheus.MustNewConstMetric(
			memoryGitHubRateLimitRemaining, prometheus.GaugeValue, float64(remain), scope,
		)
	}

	rateLimitLimit := gh.GetRateLimitLimit()
	for scope, limit := range rateLimitLimit {
		ch <- prometheus.MustNewConstMetric(
			memoryGitHubRateLimitLimiting, prometheus.GaugeValue, float64(limit), scope,
		)
	}

	return nil
}

var _ Scraper = ScraperMemory{}

func scrapeDockerValues(ch chan<- prometheus.Metric) error {
	rateLimit, err := docker.GetRateLimit()
	if err != nil {
		return fmt.Errorf("failed to get rate limit: %w", err)
	}
	ch <- prometheus.MustNewConstMetric(
		memoryDockerHubRateLimitRemaining, prometheus.GaugeValue, float64(rateLimit.Remaining),
	)
	ch <- prometheus.MustNewConstMetric(
		memoryDockerHubRateLimitLimiting, prometheus.GaugeValue, float64(rateLimit.Limit),
	)
	return nil
}
