package metric

import (
	"context"
	"fmt"

	"github.com/whywaita/myshoes/pkg/starter"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/whywaita/myshoes/pkg/datastore"
)

const memoryName = "memory"

var (
	memoryLastRunStarterUnixTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "last_run_starter_unixtime"),
		"last time of starter.do()",
		[]string{"myshoes"}, nil,
	)
	memoryErrCounterStarter = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, memoryName, "error_counter_starter"),
		"error counter of starter.do()",
		[]string{"myshoes"}, nil,
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
	if err := scrapeLastRunStarterUnixTime(ch); err != nil {
		return fmt.Errorf("failed to scrape number concurrences creating runner: %w", err)
	}
	if err := scrapeErrCounterStarter(ch); err != nil {
		return fmt.Errorf("failed to scrape error counter in starter.do: %w", err)
	}

	return nil
}

func scrapeLastRunStarterUnixTime(ch chan<- prometheus.Metric) error {
	unixtime := starter.LastRunStarterUnixTime

	result := map[string]float64{
		"myshoes": float64(unixtime),
	} // key: ???, value: unixtime
	for key, n := range result {
		ch <- prometheus.MustNewConstMetric(
			memoryLastRunStarterUnixTimeDesc, prometheus.GaugeValue, n, key,
		)
	}
	return nil
}

func scrapeErrCounterStarter(ch chan<- prometheus.Metric) error {
	errCount := starter.ErrCounterStarter

	result := map[string]float64{
		"myshoes": float64(errCount),
	} // key: ???, value: error counter
	for key, n := range result {
		ch <- prometheus.MustNewConstMetric(
			memoryErrCounterStarter, prometheus.GaugeValue, n, key,
		)
	}
	return nil
}

var _ Scraper = ScraperMemory{}
