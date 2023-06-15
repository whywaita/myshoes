package metric

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
)

const githubName = "github"

var (
	githubPendingJobsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, datastoreName, "pending_jobs"),
		"Number of pending jobs",
		[]string{"target_id"}, nil,
	)
)

type ScraperGitHub struct{}

func (ScraperGitHub) Name() string {
	return githubName
}

func (ScraperGitHub) Help() string {
	return "Collect from GitHub"
}

func (s *ScraperGitHub) Scrape(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	if err := scrapePendingJobs(ctx, ds, ch); err != nil {
		return fmt.Errorf("failed to scrape pending jobs: %w", err)
	}
	return nil
}

func scrapePendingJobs(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	targets, err := ds.ListTargets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list pending jobs: %w", err)
	}
	if len(targets) == 0 {
		ch <- prometheus.MustNewConstMetric(
			githubPendingJobsDesc, prometheus.GaugeValue, 0, "none",
		)
		return nil
	}

	result := map[string]float64{}

	for _, t := range targets {
		client, err := gh.NewClient(t.GitHubToken)
		if err != nil {
			return fmt.Errorf("failed to list pending jobs: %w", err)
		}
		owner, repo := t.OwnerRepo()
		runs, err := gh.ListRuns(ctx, client, owner, repo)
		if err != nil {
			return fmt.Errorf("failed to list pending jobs: %w", err)
		}
		if len(runs) == 0 {
			ch <- prometheus.MustNewConstMetric(
				githubPendingJobsDesc, prometheus.GaugeValue, 0, t.UUID.String(),
			)
			continue
		}
		for _, r := range runs {
			jobs, err := gh.ListJobs(ctx, client, owner, repo, r.GetID())
			if err != nil {
				return fmt.Errorf("failed to list pending jobs: %w", err)
			}
			if len(jobs) == 0 {
				ch <- prometheus.MustNewConstMetric(
					githubPendingJobsDesc, prometheus.GaugeValue, 0, t.UUID.String(),
				)
				continue
			}
			for _, j := range jobs {
				if j.GetStatus() == "pending" && time.Since(j.GetStartedAt().Time) >= 30 {
					result[t.UUID.String()]++
				}
			}
		}
	}
	for targetID, number := range result {
		ch <- prometheus.MustNewConstMetric(
			githubPendingJobsDesc, prometheus.GaugeValue, number, targetID,
		)
	}
	return nil
}
