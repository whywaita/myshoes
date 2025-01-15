package gh

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/whywaita/myshoes/pkg/logger"
)

func listInstallations(ctx context.Context) ([]*github.Installation, error) {
	if cachedRs, found := responseCache.Get(getCacheInstallationsKey()); found {
		return cachedRs.([]*github.Installation), nil
	}

	inst, err := _listInstallations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list installations: %w", err)
	}

	responseCache.Set(getCacheInstallationsKey(), inst, 1*time.Hour)

	return _listInstallations(ctx)
}

func getCacheInstallationsKey() string {
	return "installations"
}

func _listInstallations(ctx context.Context) ([]*github.Installation, error) {
	clientApps, err := NewClientGitHubApps()
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

func listAppsInstalledRepo(ctx context.Context, installationID int64) ([]*github.Repository, error) {
	if cachedRs, found := responseCache.Get(getCacheInstalledRepoKey(installationID)); found {
		return cachedRs.([]*github.Repository), nil
	}

	inst, err := _listAppsInstalledRepo(ctx, installationID)
	if err != nil {
		return nil, fmt.Errorf("failed to list installations: %w", err)
	}

	responseCache.Set(getCacheInstalledRepoKey(installationID), inst, 1*time.Hour)

	return _listAppsInstalledRepo(ctx, installationID)
}

func getCacheInstalledRepoKey(installationID int64) string {
	return fmt.Sprintf("installed-repo-%d", installationID)
}

func _listAppsInstalledRepo(ctx context.Context, installationID int64) ([]*github.Repository, error) {
	clientInstallation, err := NewClientInstallation(installationID)
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

// PurgeInstallationCache purges the cache of installations
func PurgeInstallationCache(ctx context.Context) error {
	installations, err := listInstallations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get installations: %w", err)
	}

	for _, installation := range installations {
		responseCache.Delete(getCacheInstalledRepoKey(installation.GetID()))
	}

	responseCache.Delete(getCacheInstallationsKey())
	return nil
}
