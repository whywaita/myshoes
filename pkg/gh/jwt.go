package gh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/whywaita/myshoes/pkg/logger"

	"github.com/google/go-github/v35/github"
)

// function pointers (for testing)
var (
	GHlistInstallations     = listInstallations
	GHlistAppsInstalledRepo = listAppsInstalledRepo
)

// GenerateGitHubAppsToken generate token of GitHub Apps using private key
// clientApps needs to response of `NewClientGitHubApps()`
func GenerateGitHubAppsToken(ctx context.Context, clientApps *github.Client, installationID int64, scope string) (string, *time.Time, error) {
	token, resp, err := clientApps.Apps.CreateInstallationToken(ctx, installationID, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate token from API: %w", err)
	}
	storeRateLimit(scope, resp.Rate)
	return *token.Token, token.ExpiresAt, nil
}

// IsInstalledGitHubApp check installed GitHub Apps in gheDomain + inputScope
// clientApps needs to response of `NewClientGitHubApps()`
func IsInstalledGitHubApp(ctx context.Context, gheDomain, inputScope string) (int64, error) {
	installations, err := GHlistInstallations(ctx, gheDomain)
	if err != nil {
		return -1, fmt.Errorf("failed to get list of installations: %w", err)
	}

	for _, i := range installations {
		if i.SuspendedAt != nil {
			continue
		}

		if strings.HasPrefix(inputScope, *i.Account.Login) {
			// i.Account.Login is username or Organization name.
			// e.g.) `https://github.com/example/sample` -> `example/sample`
			// strings.HasPrefix search scope include i.Account.Login.

			switch {
			case strings.EqualFold(*i.RepositorySelection, "all"):
				// "all" can use GitHub Apps in all repositories that joined i.Account.Login.
				return *i.ID, nil
			case strings.EqualFold(*i.RepositorySelection, "selected"):
				// "selected" can use GitHub Apps in only some repositories that permitted.
				// So, need to check more using other endpoint.
				err := isInstalledGitHubAppSelected(ctx, gheDomain, inputScope, *i.ID)
				if err == nil {
					// found
					return *i.ID, nil
				}
			}
		}
	}

	return -1, fmt.Errorf("%s/%s is not installed configured GitHub Apps", gheDomain, inputScope)
}

func isInstalledGitHubAppSelected(ctx context.Context, gheDomain, inputScope string, installationID int64) error {
	installedRepository, err := GHlistAppsInstalledRepo(ctx, gheDomain, installationID)
	if err != nil {
		return fmt.Errorf("failed to get list of installed repositories: %w", err)
	}

	if len(installedRepository) <= 0 {
		return fmt.Errorf("installed repository is not found")
	}

	switch DetectScope(inputScope) {
	case Organization:
		// Scope is Organization and installed repository is existed
		// So GitHub Apps installed
		return nil
	case Repository:
		for _, repo := range installedRepository {
			if strings.EqualFold(*repo.FullName, inputScope) {
				return nil
			}
		}

		return fmt.Errorf("not found")
	default:
		return fmt.Errorf("%s can't detect scope", inputScope)
	}
}

func listAppsInstalledRepo(ctx context.Context, gheDomain string, installationID int64) ([]*github.Repository, error) {
	clientInstallation, err := NewClientInstallation(gheDomain, installationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create a client installation: %w", err)
	}

	var opts = &github.ListOptions{
		Page:    0,
		PerPage: 100,
	}

	var repositories []*github.Repository
	for {
		logger.Logf(true, "get list of repository from installation, page: %d, now all repositories: %d", opts.Page, len(repositories))
		lr, resp, err := clientInstallation.Apps.ListRepos(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get installed repositories: %w", err)
		}
		repositories = append(repositories, lr.Repositories...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return repositories, nil
}

func listInstallations(ctx context.Context, gheDomain string) ([]*github.Installation, error) {
	clientApps, err := NewClientGitHubApps(gheDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to create a client Apps: %w", err)
	}

	var opts = &github.ListOptions{
		Page:    0,
		PerPage: 100,
	}

	var installations []*github.Installation
	for {
		logger.Logf(true, "get installations from GitHub, page: %d, now all installations: %d", opts.Page, len(installations))
		is, resp, err := clientApps.Apps.ListInstallations(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list installations: %w", err)
		}
		installations = append(installations, is...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return installations, nil
}
