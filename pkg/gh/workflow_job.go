package gh

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v47/github"
)

func listWorkflowJob(ctx context.Context, client *github.Client, owner, repo string, runID int64, opts *github.ListWorkflowJobsOptions) ([]*github.WorkflowJob, *github.Response, error) {
	jobs, resp, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list workflow runs: %w", err)
	}
	return jobs.Jobs, resp, nil
}

// ListWorkflowJobByRunID get workflow job by run ID
func ListWorkflowJobByRunID(ctx context.Context, client *github.Client, owner, repo string, runID int64) ([]*github.WorkflowJob, error) {
	if cachedWorkflowJobs, found := responseCache.Get(getWorkflowJobCacheKey(owner, repo, runID)); found {
		return cachedWorkflowJobs.([]*github.WorkflowJob), nil
	}
	opts := &github.ListWorkflowJobsOptions{
		Filter: "latest",
	}
	jobs, _, err := listWorkflowJob(ctx, client, owner, repo, runID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow jobs: %w", err)
	}

	responseCache.Set(getWorkflowJobCacheKey(owner, repo, runID), jobs, 1*time.Minute)
	return jobs, nil
}

func getWorkflowJobCacheKey(owner, repo string, runID int64) string {
	return fmt.Sprintf("runs-owner-%s-repo-%s-runid-%d", owner, repo, runID)
}
