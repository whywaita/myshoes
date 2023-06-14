package gh

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/whywaita/myshoes/pkg/logger"
)

func listJobs(ctx context.Context, client *github.Client, owner, repo string, runID int64, opts *github.ListWorkflowJobsOptions) (*github.Jobs, *github.Response, error) {
	jobs, resp, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list workflow jobs: %w", err)
	}
	return jobs, resp, nil
}

func ListJobs(ctx context.Context, client *github.Client, owner, repo string, runID int64) ([]*github.WorkflowJob, error) {
	if cachedJs, found := responseCache.Get(getJobsCacheKey(owner, repo)); found {
		return cachedJs.([]*github.WorkflowJob), nil
	}
	var opts = &github.ListWorkflowJobsOptions{
		ListOptions: github.ListOptions{
			Page:    0,
			PerPage: 100,
		},
	}
	var js []*github.WorkflowJob
	for {
		logger.Logf(true, "get jobs from GitHub, page: %d, now latest jobs: %d", opts.Page, len(js))
		jobs, resp, err := listJobs(ctx, client, owner, repo, runID, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list jobs: %w", err)
		}
		storeRateLimit(getRateLimitKey(owner, repo), resp.Rate)
		js = append(js, jobs.Jobs...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	responseCache.Set(getJobsCacheKey(owner, repo), js, 1*time.Second)
	logger.Logf(true, "found %d jobs in GitHub", len(js))
	return js, nil
}

func getJobsCacheKey(owner, repo string) string {
	return fmt.Sprintf("jobs-owner-%s-repo-%s", owner, repo)
}
