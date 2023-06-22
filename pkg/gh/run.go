package gh

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/whywaita/myshoes/pkg/logger"
)

func listRuns(ctx context.Context, client *github.Client, owner, repo string, opts *github.ListWorkflowRunsOptions) (*github.WorkflowRuns, *github.Response, error) {
	runs, resp, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list workflow runs: %w", err)
	}
	return runs, resp, nil
}

// ListRuns get workflow runs that registered repository
func ListRuns(ctx context.Context, owner, repo, scope string) ([]*github.WorkflowRun, error) {
	if cachedRs, expiration, found := responseCache.GetWithExpiration(getRunsCacheKey(owner, repo)); found {
		if time.Until(expiration).Minutes() <= 1 {
			go updateCache(context.Background(), owner, repo, scope)
		}
		logger.Logf(true, "found workflow runs (cache hit: expiration: %s) in %s/%s", expiration.Format("2006/01/02 15:04:05.000 -0700"), owner, repo)
		return cachedRs.([]*github.WorkflowRun), nil
	}
	go updateCache(context.Background(), owner, repo, scope)
	return []*github.WorkflowRun{}, nil
}

func getRunsCacheKey(owner, repo string) string {
	return fmt.Sprintf("runs-owner-%s-repo-%s", owner, repo)
}

func updateCache(ctx context.Context, owner, repo, scope string) {
	installationID, err := IsInstalledGitHubApp(ctx, scope)
	if err != nil {
		logger.Logf(false, "failed to list pending runs (%s/%s): %+v", owner, repo, err)
		installationID, err = IsInstalledGitHubApp(ctx, owner)
		if err != nil {
			logger.Logf(false, "failed to list pending runs (%s): %+v", owner, err)
			responseCache.Set(getRunsCacheKey(owner, repo), []*github.WorkflowRun{}, 1*time.Hour)
			return
		}
	}
	client, err := NewClientInstallation(installationID)
	if err != nil {
		logger.Logf(false, "failed to list workflow runs (%s/%s): %+v", owner, repo, err)
		return
	}
	var opts = &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{
			Page:    0,
			PerPage: 10,
		},
	}
	logger.Logf(true, "get workflow runs of %s/%s, recent %d runs", owner, repo, opts.PerPage)
	runs, resp, err := listRuns(ctx, client, owner, repo, opts)
	if err != nil {
		logger.Logf(false, "failed to list workflow runs (%s/%s): %+v", owner, repo, err)
		return
	}
	storeRateLimit(getRateLimitKey(owner, repo), resp.Rate)
	responseCache.Set(getRunsCacheKey(owner, repo), runs.WorkflowRuns, 15*time.Minute)
	logger.Logf(true, "found %d workflow runs in %s/%s", len(runs.WorkflowRuns), owner, repo)
}
