package gh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/whywaita/myshoes/pkg/logger"
)

// ExistGitHubRunner check exist registered of GitHub runner
func ExistGitHubRunner(ctx context.Context, client *github.Client, owner, repo, runnerName string) (*github.Runner, error) {
	runners, err := ListRunners(ctx, client, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of runners: %w", err)
	}

	return ExistGitHubRunnerWithRunner(runners, runnerName)
}

// ExistGitHubRunnerWithRunner check exist registered of GitHub runner from a list of runner
func ExistGitHubRunnerWithRunner(runners []*github.Runner, runnerName string) (*github.Runner, error) {
	for _, r := range runners {
		if strings.EqualFold(r.GetName(), runnerName) {
			return r, nil
		}
	}

	return nil, ErrNotFound
}

// ListRunners get runners that registered repository or org
func ListRunners(ctx context.Context, client *github.Client, owner, repo string) ([]*github.Runner, error) {
	if cachedRs, found := responseCache.Get(getCacheKey(owner, repo)); found {
		return cachedRs.([]*github.Runner), nil
	}

	var opts = &github.ListOptions{
		Page:    0,
		PerPage: 100,
	}

	var rs []*github.Runner
	for {
		logger.Logf(true, "get runners from GitHub, page: %d, now all runners: %d", opts.Page, len(rs))
		runners, resp, err := listRunners(ctx, client, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list runners: %w", err)
		}
		storeRateLimit(getRateLimitKey(owner, repo), resp.Rate)

		rs = append(rs, runners.Runners...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	responseCache.Set(getCacheKey(owner, repo), rs, 1*time.Second)
	logger.Logf(true, "found %d runners in GitHub", len(rs))

	return rs, nil
}

func getCacheKey(owner, repo string) string {
	return fmt.Sprintf("owner-%s-repo-%s", owner, repo)
}

func listRunners(ctx context.Context, client *github.Client, owner, repo string, opts *github.ListOptions) (*github.Runners, *github.Response, error) {
	if repo == "" {
		runners, resp, err := client.Actions.ListOrganizationRunners(ctx, owner, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list organization runners: %w", err)
		}
		return runners, resp, nil
	}

	runners, resp, err := client.Actions.ListRunners(ctx, owner, repo, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list repository runners: %w", err)
	}
	return runners, resp, nil
}

// GetLatestRunnerVersion get a latest version of actions/runner
func GetLatestRunnerVersion(ctx context.Context, scope string) (string, error) {
	clientApps, err := NewClientGitHubApps()
	if err != nil {
		return "", fmt.Errorf("failed to create a client from Apps: %+v", err)
	}
	installationID, err := IsInstalledGitHubApp(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("failed to get installlation id: %w", err)
	}
	token, _, err := GenerateGitHubAppsToken(ctx, clientApps, installationID, scope)
	if err != nil {
		return "", fmt.Errorf("failed to get registration token: %w", err)
	}
	client, err := NewClient(token)
	if err != nil {
		return "", fmt.Errorf("failed to create GitHub client: %w", err)
	}

	switch DetectScope(scope) {
	case Repository:
		owner, repo := DivideScope(scope)
		applications, resp, err := client.Actions.ListRunnerApplicationDownloads(ctx, owner, repo)
		if err != nil {
			return "", fmt.Errorf("failed to get latest runner version: %w", err)
		}
		storeRateLimit(getRateLimitKey(owner, repo), resp.Rate)
		return getRunnerVersion(applications)
	case Organization:
		applications, resp, err := client.Actions.ListOrganizationRunnerApplicationDownloads(ctx, scope)
		if err != nil {
			return "", fmt.Errorf("failed to get latest runner version: %w", err)
		}
		storeRateLimit(getRateLimitKey(scope, ""), resp.Rate)
		return getRunnerVersion(applications)
	}
	return "", fmt.Errorf("invalid scope: %s", scope)
}

func getRunnerVersion(applications []*github.RunnerApplicationDownload) (string, error) {
	// filename": "actions-runner-linux-x64-2.164.0.tar.gz"
	for _, app := range applications {
		if *app.OS == "linux" && *app.Architecture == "x64" {
			v := strings.ReplaceAll(*app.Filename, "actions-runner-linux-x64-", "")
			v = strings.ReplaceAll(v, ".tar.gz", "")
			return fmt.Sprintf("v%s", v), nil
		}
	}

	return "", fmt.Errorf("not found runner version")
}

// ConcatLabels concat labels from check event JSON
func ConcatLabels(checkEventJSON string) (string, error) {
	runsOnLabels, err := ExtractRunsOnLabels([]byte(checkEventJSON))
	if err != nil {
		return "", fmt.Errorf("failed to extract runs-on labels: %w", err)
	}

	runsOnConcat := "none"
	if len(runsOnLabels) != 0 {
		runsOnConcat = strings.Join(runsOnLabels, ",") // e.g. "self-hosted,linux"
	}
	return runsOnConcat, nil
}
