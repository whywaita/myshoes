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
	githubInstallationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, githubName, "installation"),
		"installations",
		[]string{
			"installation_id",
			"account_login",
			"account_type",
			"target_type",
			"repository_selection",
			"html_url",
		},
		nil,
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
	if err := scrapeInstallation(ctx, ch); err != nil {
		return fmt.Errorf("failed to scrape installations: %w", err)
	}
	return nil
}

func scrapePendingRuns(ctx context.Context, ds datastore.Datastore, ch chan<- prometheus.Metric) error {
	gh.ActiveTargets.Range(func(key, value any) bool {
		var pendings float64
		repoName := key.(string)
		installationID := value.(int64)
		target, err := datastore.SearchRepo(ctx, ds, repoName)
		if err != nil {
			logger.Logf(false, "failed to scrape pending run: failed to get target by scope (%s): %+v", repoName, err)
			return true
		}
		owner, repo := target.OwnerRepo()
		if repo == "" {
			return true
		}
		runs, err := gh.ListRuns(owner, repo)
		if err != nil {
			logger.Logf(false, "failed to scrape pending run: failed to list pending runs: %+v", err)
			return true
		}

		for _, r := range runs {
			if r.GetStatus() == "queued" || r.GetStatus() == "pending" {
				oldMinutes := 30
				sinceMinutes := time.Since(r.CreatedAt.Time).Minutes()
				if sinceMinutes >= float64(oldMinutes) {
					logger.Logf(false, "run %d is pending over %d minutes, So will enqueue", r.GetID(), oldMinutes)
					pendings++
					gh.PendingRuns.Store(installationID, r)
				} else {
					logger.Logf(true, "run %d is pending, but not over %d minutes. So ignore (since: %f minutes)", r.GetID(), oldMinutes, sinceMinutes)
				}
			}
		}
		ch <- prometheus.MustNewConstMetric(githubPendingRunsDesc, prometheus.GaugeValue, pendings, target.UUID.String(), target.Scope)
		return true
	})
	return nil
}

func scrapeInstallation(ctx context.Context, ch chan<- prometheus.Metric) error {
	installations, err := gh.GHlistInstallations(ctx)
	if err != nil {
		return fmt.Errorf("failed to list installations: %w", err)
	}

	for _, installation := range installations {
		ch <- prometheus.MustNewConstMetric(
			githubInstallationDesc,
			prometheus.GaugeValue,
			1,
			fmt.Sprint(installation.GetID()),
			installation.GetAccount().GetLogin(),
			installation.GetAccount().GetType(),
			installation.GetTargetType(),
			installation.GetRepositorySelection(),
			installation.GetAccount().GetHTMLURL(),
		)
	}
	return nil
}
