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
		[]string{"target_id"}, nil,
	)
	datastoreTargetsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "targets"),
		"Number of targets",
		[]string{"resource_type"}, nil,
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
	if err := scrapeJobs(ctx, ds, ch); err != nil {
		return fmt.Errorf("failed to scrape jobs: %w", err)
	}
	if err := scrapeTargets(ctx, ds, ch); err != nil {
		return fmt.Errorf("failed to scrape targets: %w", err)
	}

	return nil
}

func scrapeJobs(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	jobs, err := ds.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	if len(jobs) == 0 {
		ch <- prometheus.MustNewConstMetric(
			datastoreJobsDesc, prometheus.GaugeValue, 0, "none",
		)
		return nil
	}

	result := map[string]float64{} // key: target_id, value: number
	for _, j := range jobs {
		result[j.TargetID.String()]++
	}
	for targetID, number := range result {
		ch <- prometheus.MustNewConstMetric(
			datastoreJobsDesc, prometheus.GaugeValue, number, targetID,
		)
	}

	return nil
}

func scrapeTargets(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	targets, err := datastore.ListTargets(ctx, ds)
	if err != nil {
		return fmt.Errorf("failed to list targets: %w", err)
	}

	result := map[string]float64{} // key: resource_type, value: number
	for _, t := range targets {
		result[t.ResourceType.String()]++
	}
	for rt, number := range result {
		ch <- prometheus.MustNewConstMetric(
			datastoreTargetsDesc, prometheus.GaugeValue, number, rt,
		)
	}

	return nil
}

var _ Scraper = ScraperDatastore{}
