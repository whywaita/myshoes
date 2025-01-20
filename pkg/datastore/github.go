package datastore

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/whywaita/myshoes/pkg/logger"

	"github.com/google/go-github/v47/github"
	"github.com/whywaita/myshoes/pkg/gh"
)

// NewClientInstallationByRepo create a client of GitHub using installation ID from repo name
func NewClientInstallationByRepo(ctx context.Context, ds Datastore, repo string) (*github.Client, *Target, error) {
	target, err := SearchRepo(ctx, ds, repo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search repository: %w", err)
	}

	installationID, err := gh.IsInstalledGitHubApp(ctx, target.Scope)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get installation ID: %w", err)
	}

	client, err := gh.NewClientInstallation(installationID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client: %w", err)
	}

	return client, target, nil
}

// PendingWorkflowRunWithTarget is struct for pending workflow run
type PendingWorkflowRunWithTarget struct {
	Target      *Target
	WorkflowRun *github.WorkflowRun
}

// GetPendingWorkflowRunByRecentRepositories get pending workflow runs by recent active repositories
func GetPendingWorkflowRunByRecentRepositories(ctx context.Context, ds Datastore) ([]PendingWorkflowRunWithTarget, error) {
	pendingRuns, err := getPendingWorkflowRunByRecentRepositories(ctx, ds)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending workflow runs: %w", err)
	}

	queuedJob, err := ds.ListJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of jobs: %w", err)
	}

	var result []PendingWorkflowRunWithTarget
	// We ignore the pending run if the job is already queued.
	for _, pendingRun := range pendingRuns {
		for _, job := range queuedJob {
			webhookEvent, err := github.ParseWebHook("workflow_job", []byte(job.CheckEventJSON))
			if err != nil {
				logger.Logf(false, "failed to parse webhook payload (job id: %s): %+v", job.UUID, err)
				continue
			}

			workflowJob, ok := webhookEvent.(*github.WorkflowJobEvent)
			if !ok {
				logger.Logf(false, "failed to cast to WorkflowJobEvent (job id: %s)", job.UUID)
				continue
			}

			if pendingRun.WorkflowRun.GetID() == workflowJob.GetWorkflowJob().GetRunID() {
				logger.Logf(true, "found job in datastore, So will ignore: (repo: %s, runID: %d)", pendingRun.WorkflowRun.GetRepository().GetFullName(), pendingRun.WorkflowRun.GetID())
				continue
			}
		}

		result = append(result, pendingRun)
	}

	return result, nil
}

func getPendingWorkflowRunByRecentRepositories(ctx context.Context, ds Datastore) ([]PendingWorkflowRunWithTarget, error) {
	recentActiveRepositories, err := getRecentRepositories(ctx, ds)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent repositories: %w", err)
	}

	var pendingRuns []PendingWorkflowRunWithTarget
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, repoRawURL := range recentActiveRepositories {
		wg.Add(1)
		go func(repoRawURL string) {
			defer wg.Done()
			u, err := url.Parse(repoRawURL)
			if err != nil {
				logger.Logf(false, "failed to get pending run by recent repositories: failed to parse repository url: %+v", err)
				return
			}
			fullName := strings.TrimPrefix(u.Path, "/")
			client, target, err := NewClientInstallationByRepo(ctx, ds, fullName)
			if err != nil {
				logger.Logf(false, "failed to get pending run by recent repositories: failed to create a client of GitHub by repo (full_name: %s) %+v", fullName, err)
				return
			}

			owner, repo := gh.DivideScope(fullName)
			pendingRunsByRepo, err := getPendingRunByRepo(ctx, client, owner, repo)
			if err != nil {
				logger.Logf(false, "failed to get pending run by recent repositories: failed to get pending run by repo (full_name: %s) %+v", fullName, err)
				return
			}
			mu.Lock()
			for _, run := range pendingRunsByRepo {
				pendingRuns = append(pendingRuns, PendingWorkflowRunWithTarget{
					Target:      target,
					WorkflowRun: run,
				})
			}
			mu.Unlock()
		}(repoRawURL)
	}

	wg.Wait()

	return pendingRuns, nil
}

func getPendingRunByRepo(ctx context.Context, client *github.Client, owner, repo string) ([]*github.WorkflowRun, error) {
	runs, err := gh.ListWorkflowRunsNewest(ctx, client, owner, repo, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}

	var pendingRuns []*github.WorkflowRun
	for _, r := range runs {
		if r.GetStatus() == "queued" || r.GetStatus() == "pending" {
			oldMinutes := 10
			sinceMinutes := time.Since(r.CreatedAt.Time).Minutes()
			if sinceMinutes >= float64(oldMinutes) {
				logger.Logf(false, "run %d is pending over %d minutes, So will enqueue (repo: %s/%s)", r.GetID(), oldMinutes, owner, repo)
				pendingRuns = append(pendingRuns, r)
			} else {
				logger.Logf(true, "run %d is pending, but not over %d minutes. So ignore (since: %f minutes, repo: %s/%s)", r.GetID(), oldMinutes, sinceMinutes, owner, repo)
			}
		}
	}

	return pendingRuns, nil
}

func getRecentRepositories(ctx context.Context, ds Datastore) ([]string, error) {
	recent := time.Now().Add(-1 * time.Hour)
	recentRunners, err := ds.ListRunnersLogBySince(ctx, recent)
	if err != nil {
		return nil, fmt.Errorf("failed to get targets from datastore: %w", err)
	}

	// sort by created_at
	sort.SliceStable(recentRunners, func(i, j int) bool {
		return recentRunners[i].CreatedAt.After(recentRunners[j].CreatedAt)
	})

	// unique repositories
	recentActiveRepositories := make(map[string]struct{})
	for _, r := range recentRunners {
		u := r.RepositoryURL
		if _, ok := recentActiveRepositories[u]; !ok {
			recentActiveRepositories[u] = struct{}{}
		}
	}
	var result []string
	for repository := range recentActiveRepositories {
		result = append(result, repository)
	}

	return result, nil
}
