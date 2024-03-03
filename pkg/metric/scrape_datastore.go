package metric

import (
	"context"
	"fmt"
	"sort"
	"strconv"
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
		[]string{"target_id", "runs_on", "oldest_created_at_duration_seconds"}, nil,
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

	type storedValue struct {
		oldestCreatedAt *time.Time
		Count           float64
	}

	stored := map[string]storedValue{}
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
		v, ok := stored[key]
		if !ok {
			stored[key] = storedValue{
				oldestCreatedAt: &j.CreatedAt,
				Count:           1,
			}
		} else {
			if v.oldestCreatedAt == nil || j.CreatedAt.Before(*v.oldestCreatedAt) {
				stored[key] = storedValue{
					oldestCreatedAt: &j.CreatedAt,
					Count:           v.Count + 1,
				}
			} else {
				stored[key] = storedValue{
					oldestCreatedAt: v.oldestCreatedAt,
					Count:           v.Count + 1,
				}
			}
		}
	}
	for key, value := range stored {
		// key: target_id-_-runs-on
		// value: storedValue

		duration := strconv.FormatFloat(time.Since(*value.oldestCreatedAt).Seconds(), 'f', -1, 64)

		split := strings.Split(key, "-_-")
		ch <- prometheus.MustNewConstMetric(
			datastoreJobsDesc, prometheus.GaugeValue,
			value.Count,
			split[0], // target_id
			split[1], // runs-on
			duration, // oldest created_at
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
