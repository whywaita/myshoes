package metric

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
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
	memoryGitHubRateLimit = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "github_rate_limit"),
		"The number of rate limit",
		[]string{"scope"}, nil,
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

	return nil
}

func scrapeStarterValues(ch chan<- prometheus.Metric) error {
	configMax := config.Config.MaxConnectionsToBackend

	const labelStarter = "starter"

	ch <- prometheus.MustNewConstMetric(
		memoryStarterMaxRunning, prometheus.GaugeValue, float64(configMax), labelStarter)

	countRunning := starter.CountRunning
	countWaiting := starter.CountWaiting

	ch <- prometheus.MustNewConstMetric(
		memoryStarterQueueRunning, prometheus.GaugeValue, float64(countRunning), labelStarter)
	ch <- prometheus.MustNewConstMetric(
		memoryStarterQueueWaiting, prometheus.GaugeValue, float64(countWaiting), labelStarter)

	return nil
}

func scrapeGitHubValues(ch chan<- prometheus.Metric) error {
	rateLimitCount := gh.GetRateLimit()
	for scope, count := range rateLimitCount {
		ch <- prometheus.MustNewConstMetric(
			memoryGitHubRateLimit, prometheus.GaugeValue, float64(count), scope,
		)
	}

	return nil
}

var _ Scraper = ScraperMemory{}
