package gh

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/sync/singleflight"

	"github.com/google/go-github/v35/github"
	"github.com/whywaita/myshoes/pkg/logger"
)

var groupRunner singleflight.Group

// ExistGitHubRunner check exist registered of GitHub runner
func ExistGitHubRunner(ctx context.Context, client *github.Client, owner, repo, runnerName string) (*github.Runner, error) {
	runners, err := ListRunners(ctx, client, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of runners: %w", err)
	}

	for _, r := range runners {
		if strings.EqualFold(r.GetName(), runnerName) {
			return r, nil
		}
	}

	return nil, ErrNotFound
}

// ListRunners get runners that registered repository or org
func ListRunners(ctx context.Context, client *github.Client, owner, repo string) ([]*github.Runner, error) {
	v, err, _ := groupRunner.Do(fmt.Sprintf("%s/%s", owner, repo), func() (interface{}, error) {
		runners, err := listRunners(ctx, client, owner, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to get a list of runner: %w", err)
		}
		return runners, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute listRunners using singleflight: %w", err)
	}

	runners, ok := v.([]*github.Runner)
	if !ok {
		return nil, fmt.Errorf("failed to cast type of runner")
	}

	return runners, err
}

func listRunners(ctx context.Context, client *github.Client, owner, repo string) ([]*github.Runner, error) {
	var opts = &github.ListOptions{
		Page:    0,
		PerPage: 10,
	}

	var rs []*github.Runner
	for {
		logger.Logf(true, "get runners from GitHub, page: %d, now all runners: %d", opts.Page, len(rs))
		runners, resp, err := callListRunners(ctx, client, owner, repo, opts)
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

	logger.Logf(true, "found %d runners in GitHub", len(rs))

	return rs, nil
}

func callListRunners(ctx context.Context, client *github.Client, owner, repo string, opts *github.ListOptions) (*github.Runners, *github.Response, error) {
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
