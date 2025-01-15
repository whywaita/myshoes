package gh

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/whywaita/myshoes/pkg/logger"
)

func listWorkflowRuns(ctx context.Context, client *github.Client, owner, repo string, opts *github.ListWorkflowRunsOptions) (*github.WorkflowRuns, *github.Response, error) {
	runs, resp, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list workflow runs: %w", err)
	}
	return runs, resp, nil
}

// ListWorkflowRunsNewest get workflow runs that registered in the last (%d: limit) runs
func ListWorkflowRunsNewest(ctx context.Context, client *github.Client, owner, repo string, limit int) ([]*github.WorkflowRun, error) {
	if cachedWorkflowRuns, found := responseCache.Get(getRunsCacheKey(owner, repo)); found {
		return cachedWorkflowRuns.([]*github.WorkflowRun), nil
	}

	var opts = &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{
			Page:    0,
			PerPage: 10,
		},
	}

	var workflowRuns []*github.WorkflowRun
	for {
		logger.Logf(true, "get workflow runs from GitHub, page: %d, now all runners: %d", opts.Page, len(workflowRuns))
		runs, resp, err := listWorkflowRuns(ctx, client, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list workflow runs: %w", err)
		}
		storeRateLimit(getRateLimitKey(owner, repo), resp.Rate)

		workflowRuns = append(workflowRuns, runs.WorkflowRuns...)
		if resp.NextPage == 0 {
			break
		}
		if len(workflowRuns) >= limit {
			break
		}

		opts.Page = resp.NextPage
	}

	responseCache.Set(getRunsCacheKey(owner, repo), workflowRuns, 3*time.Minute)
	return workflowRuns, nil
}

func getRunsCacheKey(owner, repo string) string {
	return fmt.Sprintf("runs-owner-%s-repo-%s", owner, repo)
}
