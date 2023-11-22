package metric

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
)

const datastoreName = "datastore"

var (
	datastoreJobsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "jobs"),
		"Number of jobs",
		[]string{"target_id", "runs_on"}, nil,
	)
	datastoreTargetsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "targets"),
		"Number of targets",
		[]string{"resource_type"}, nil,
	)
	datastoreJobDurationOldest = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "job_duration_oldest_seconds"),
		"Duration time of oldest job",
		[]string{"job_id"}, nil,
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
			datastoreJobsDesc, prometheus.GaugeValue, 0, "none", "none",
		)
		return nil
	}

	sort.SliceStable(jobs, func(i, j int) bool {
		// oldest job is first
		return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
	})

	oldestJob := jobs[0]
	ch <- prometheus.MustNewConstMetric(
		datastoreJobDurationOldest, prometheus.GaugeValue, time.Since(oldestJob.CreatedAt).Seconds(), oldestJob.UUID.String())

	stored := map[string]float64{}
	// job separate target_id and runs-on labels
	for _, j := range jobs {
		runsOnLabels, err := gh.ExtractRunsOnLabels([]byte(j.CheckEventJSON))
		if err != nil {
			return fmt.Errorf("failed to extract runs-on labels: %w", err)
		}

		runsOnConcat := "none"
		if len(runsOnLabels) != 0 {
			runsOnConcat = strings.Join(runsOnLabels, ",") // e.g. "self-hosted,linux"
		}
		key := fmt.Sprintf("%s-_-%s", j.TargetID.String(), runsOnConcat)
		stored[key]++
	}
	for key, number := range stored {
		// key: target_id-_-runs-on
		// value: number of jobs
		split := strings.Split(key, "-_-")
		ch <- prometheus.MustNewConstMetric(
			datastoreJobsDesc, prometheus.GaugeValue, number, split[0], split[1],
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
