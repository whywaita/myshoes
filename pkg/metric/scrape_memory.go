package metric

import (
	"context"
	"fmt"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/starter"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/whywaita/myshoes/pkg/datastore"
)

const memoryName = "memory"

var (
	memoryStarterStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "starter_status"),
		"status values from starter",
		[]string{"type"}, nil,
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

	return nil
}

func scrapeStarterValues(ch chan<- prometheus.Metric) error {
	maxConnections := config.Config.MaxConnectionsToBackend
	connections := starter.ConnectionsSemaphore

	result := map[string]float64{
		"max_connections":  float64(maxConnections),
		"semaphore_number": float64(connections),
	}
	for key, n := range result {
		ch <- prometheus.MustNewConstMetric(
			memoryStarterStatus, prometheus.GaugeValue, n, key)
	}
	return nil
}

var _ Scraper = ScraperMemory{}
