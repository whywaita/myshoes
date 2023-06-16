package metric

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
)

const githubName = "github"

var (
	githubPendingRunsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, githubName, "pending_runs"),
		"Number of pending runs",
		[]string{"target_id"}, nil,
	)
)

// ScraperGitHub is scraper implement for GitHub
type ScraperGitHub struct{}

// Name return name
func (ScraperGitHub) Name() string {
	return githubName
}

// Help return help
func (ScraperGitHub) Help() string {
	return "Collect from GitHub"
}

// Scrape scrape metrics
func (s ScraperGitHub) Scrape(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	if err := scrapePendingRuns(ctx, ds, ch); err != nil {
		return fmt.Errorf("failed to scrape pending runs: %w", err)
	}
	return nil
}

func scrapePendingRuns(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	targets, err := ds.ListTargets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list pending runs: %w", err)
	}
	if len(targets) == 0 {
		ch <- prometheus.MustNewConstMetric(
			githubPendingRunsDesc, prometheus.GaugeValue, 0, "none",
		)
		return nil
	}

	for _, t := range targets {
		owner, repo := t.OwnerRepo()
		var pendings float64
		if repo == "" {
			continue
		}
		runs, err := gh.ListRuns(ctx, owner, repo, t.Scope)
		if err != nil {
			logger.Logf(false, "failed to list pending runs: %+v", err)
			continue
		}

		if len(runs) == 0 {
			ch <- prometheus.MustNewConstMetric(
				githubPendingRunsDesc, prometheus.GaugeValue, 0, t.UUID.String(),
			)
			continue
		}
		for _, r := range runs {
			if r.GetStatus() == "queued" || r.GetStatus() == "pending" {
				if time.Since(r.CreatedAt.Time).Minutes() >= 30 {
					pendings++
				}
			}
		}
		ch <- prometheus.MustNewConstMetric(
			githubPendingRunsDesc, prometheus.GaugeValue, pendings, t.UUID.String(),
		)
	}
	return nil
}
