package metric

import (
	"context"
	"fmt"
	"strconv"
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
	githubPendingWorkflowRunSecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, githubName, "pending_workflow_run_seconds"),
		"Second of Pending time in workflow run",
		[]string{"target_id", "workflow_id", "workflow_run_id", "html_url"}, nil,
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
	pendingRuns, err := datastore.GetPendingWorkflowRunByRecentRepositories(ctx, ds)
	if err != nil {
		return fmt.Errorf("failed to get pending workflow runs: %w", err)
	}

	for _, pendingRun := range pendingRuns {
		sinceSeconds := time.Since(pendingRun.WorkflowRun.CreatedAt.Time).Seconds()

		ch <- prometheus.MustNewConstMetric(
			githubPendingWorkflowRunSecondsDesc,
			prometheus.GaugeValue,
			sinceSeconds,
			pendingRun.Target.UUID.String(),
			strconv.FormatInt(pendingRun.WorkflowRun.GetWorkflowID(), 10),
			strconv.FormatInt(pendingRun.WorkflowRun.GetID(), 10),
			pendingRun.WorkflowRun.GetHTMLURL(),
		)
	}

	// count pending runs by target
	countPendingMap := make(map[string]int)
	targetCache := make(map[string]*datastore.Target)
	for _, pendingRun := range pendingRuns {
		countPendingMap[pendingRun.Target.UUID.String()]++
		targetCache[pendingRun.Target.UUID.String()] = pendingRun.Target
	}

	for targetID, countPending := range countPendingMap {
		target, ok := targetCache[targetID]
		if !ok {
			logger.Logf(false, "failed to get target by targetID from targetCache: %s", targetID)
			continue
		}
		ch <- prometheus.MustNewConstMetric(githubPendingRunsDesc, prometheus.GaugeValue, float64(countPending), target.UUID.String(), target.Scope)
	}
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
