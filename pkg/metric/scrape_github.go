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
		[]string{"target_id", "scope"}, nil,
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
	gh.ActiveTargets.Range(func(key, value any) bool {
		var pendings float64
		scope := key.(string)
		installationID := value.(int64)
		target, err := ds.GetTargetByScope(ctx, scope)
		if err != nil {
			logger.Logf(false, "failed to get target by scope (%s): %+v", scope, err)
			return true
		}
		owner, repo := target.OwnerRepo()
		if repo == "" {
			return true
		}
		runs, err := gh.ListRuns(owner, repo)
		if err != nil {
			logger.Logf(false, "failed to list pending runs: %+v", err)
			return true
		}

		for _, r := range runs {
			if r.GetStatus() == "queued" || r.GetStatus() == "pending" {
				if time.Since(r.CreatedAt.Time).Minutes() >= 30 {
					pendings++
					gh.PendingRuns.Store(installationID, r)
				}
			}
		}
		ch <- prometheus.MustNewConstMetric(githubPendingRunsDesc, prometheus.GaugeValue, pendings, target.UUID.String(), target.Scope)
		return true
	})
	return nil
}
