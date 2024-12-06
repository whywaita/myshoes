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
