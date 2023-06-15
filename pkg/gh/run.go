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

func ListRuns(ctx context.Context, client *github.Client, owner, repo string) ([]*github.WorkflowRun, error) {
	if cachedRs, found := responseCache.Get(getRunsCacheKey(owner, repo)); found {
		return cachedRs.([]*github.WorkflowRun), nil
	}
	var opts = &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{
			Page:    0,
			PerPage: 10,
		},
	}
	logger.Logf(true, "get workflow runs from GitHub, now recent %d runs", opts.PerPage)
	runs, resp, err := listRuns(ctx, client, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow runs: %w", err)
	}
	storeRateLimit(getRateLimitKey(owner, repo), resp.Rate)
	responseCache.Set(getRunsCacheKey(owner, repo), runs.WorkflowRuns, 1*time.Second)
	logger.Logf(true, "found %d workflow runs in GitHub", len(runs.WorkflowRuns))
	return runs.WorkflowRuns, nil
}

func getRunsCacheKey(owner, repo string) string {
	return fmt.Sprintf("runs-owner-%s-repo-%s", owner, repo)
}
