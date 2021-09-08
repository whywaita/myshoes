package metric

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/whywaita/myshoes/pkg/datastore"
)

const datastoreName = "datastore"

var (
	datastoreJobsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "jobs"),
		"Number of jobs",
		[]string{"key"}, nil,
	)
)

// ScraperDatastore is scraper implement for datastore.Datastore
type ScraperDatastore struct{}

// Name return name
func (ScraperDatastore) Name() string {
	return datastoreName
}

// Help return help
func (ScraperDatastore) Help() string {
	return "Collect from datastore"
}

// Scrape scrape metrics
func (ScraperDatastore) Scrape(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	jobs, err := ds.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}
	ch <- prometheus.MustNewConstMetric(
		datastoreJobsDesc, prometheus.GaugeValue, float64(len(jobs)), "value",
	)

	return nil
}

var _ Scraper = ScraperDatastore{}
