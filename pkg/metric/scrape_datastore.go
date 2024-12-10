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
	"github.com/whywaita/myshoes/pkg/logger"
	"github.com/whywaita/myshoes/pkg/starter"
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
	datastoreTargetDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "target_describe"),
		"Target",
		[]string{
			"target_id",
			"scope",
			"resource_type",
		}, nil,
	)
	datastoreTargetTokenExpiresDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "target_token_expires_seconds"),
		"Token expires time",
		[]string{"target_id"}, nil,
	)
	datastoreJobDurationOldest = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "job_duration_oldest_seconds"),
		"Duration time of oldest job",
		[]string{"job_id", "runs_on"}, nil,
	)
	datastoreDeletedJobsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "deleted_jobs"),
		"Number of deleted jobs",
		[]string{"runs_on"}, nil,
	)
	datastoreRunnersRunningDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "runners_running"),
		"Number of runners running",
		[]string{"target_id"}, nil,
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
	if err := scrapeRunners(ctx, ds, ch); err != nil {
		return fmt.Errorf("failed to scrape runners: %w", err)
	}

	return nil
}

func scrapeJobs(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	if err := scrapeJobCounter(ctx, ds, ch); err != nil {
		return fmt.Errorf("failed to scrape job counter: %w", err)
	}

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
	type storedValue struct {
		OldestJob datastore.Job
		Count     float64
	}

	stored := map[string]storedValue{}
	// job separate target_id and runs-on labels
	for _, j := range jobs {
		runsOnConcat, err := gh.ConcatLabels(j.CheckEventJSON)
		if err != nil {
			logger.Logf(false, "failed to concat labels: %+v", err)
			continue
		}
		key := fmt.Sprintf("%s-_-%s", j.TargetID.String(), runsOnConcat)
		v, ok := stored[key]
		if !ok {
			stored[key] = storedValue{
				OldestJob: j,
				Count:     1,
			}
		} else {
			if j.CreatedAt.Before(v.OldestJob.CreatedAt) {
				stored[key] = storedValue{
					OldestJob: j,
					Count:     v.Count + 1,
				}
			} else {
				stored[key] = storedValue{
					OldestJob: v.OldestJob,
					Count:     v.Count + 1,
				}
			}
		}
	}
	for key, value := range stored {
		// key: target_id-_-runs-on
		// value: storedValue

		split := strings.Split(key, "-_-")
		ch <- prometheus.MustNewConstMetric(
			datastoreJobsDesc, prometheus.GaugeValue,
			value.Count,
			split[0], // target_id
			split[1], // runs-on
		)

		ch <- prometheus.MustNewConstMetric(
			datastoreJobDurationOldest,
			prometheus.GaugeValue,
			time.Since(value.OldestJob.CreatedAt).Seconds(),
			value.OldestJob.UUID.String(),
			split[1],
		)
	}

	return nil
}

func scrapeJobCounter(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	starter.DeletedJobMap.Range(func(key, value interface{}) bool {
		runsOn := key.(string)
		number := value.(int)
		ch <- prometheus.MustNewConstMetric(
			datastoreDeletedJobsDesc, prometheus.CounterValue, float64(number), runsOn,
		)
		return true
	})
	return nil
}

func scrapeTargets(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	targets, err := datastore.ListTargets(ctx, ds)
	if err != nil {
		return fmt.Errorf("failed to list targets: %w", err)
	}

	now := time.Now()
	result := map[string]float64{} // key: resource_type, value: number
	for _, t := range targets {
		ch <- prometheus.MustNewConstMetric(
			datastoreTargetDesc, prometheus.GaugeValue, 1,
			t.UUID.String(), t.Scope, t.ResourceType.String(),
		)

		result[t.ResourceType.String()]++

		ch <- prometheus.MustNewConstMetric(
			datastoreTargetTokenExpiresDesc, prometheus.GaugeValue,
			t.TokenExpiredAt.Sub(now).Seconds(),
			t.UUID.String(),
		)
	}
	for rt, number := range result {
		ch <- prometheus.MustNewConstMetric(
			datastoreTargetsDesc, prometheus.GaugeValue, number, rt,
		)
	}

	return nil
}

func scrapeRunners(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	runners, err := ds.ListRunners(ctx)
	if err != nil {
		return fmt.Errorf("failed to list runners: %w", err)
	}

	result := map[string]float64{} // key: target_id, value: number
	for _, r := range runners {
		result[r.TargetID.String()]++
	}
	for targetID, number := range result {
		ch <- prometheus.MustNewConstMetric(
			datastoreRunnersRunningDesc, prometheus.GaugeValue, number, targetID,
		)
	}

	return nil
}

var _ Scraper = ScraperDatastore{}
