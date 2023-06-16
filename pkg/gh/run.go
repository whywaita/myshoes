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
	if cachedRs, found := responseCache.Get(getRunsCacheKey(owner, repo)); found {
		logger.Logf(true, "found workflow runs (cache hit) in %s/%s", owner, repo)
		return cachedRs.([]*github.WorkflowRun), nil
	}
	installationID, err := IsInstalledGitHubApp(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending runs (%s/%s): %w", owner, repo, err)
	}
	client, err := NewClientInstallation(installationID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow runs (%s/%s): %w", owner, repo, err)
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
		return nil, fmt.Errorf("failed to list workflow runs (%s/%s): %w", owner, repo, err)
	}
	storeRateLimit(getRateLimitKey(owner, repo), resp.Rate)
	responseCache.Set(getRunsCacheKey(owner, repo), runs.WorkflowRuns, 5*time.Minute)
	logger.Logf(true, "found %d workflow runs in %s/%s", len(runs.WorkflowRuns), owner, repo)
	return runs.WorkflowRuns, nil
}

func getRunsCacheKey(owner, repo string) string {
	return fmt.Sprintf("runs-owner-%s-repo-%s", owner, repo)
}
