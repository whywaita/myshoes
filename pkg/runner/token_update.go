package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
)

func (m *Manager) doTargetToken(ctx context.Context) error {
	logger.Logf(true, "start refresh token")

	targets, err := datastore.ListTargets(ctx, m.ds)
	if err != nil {
		return fmt.Errorf("failed to get targets: %w", err)
	}

	for _, target := range targets {
		needRefreshTime := target.TokenExpiredAt.Add(-1 * NeedRefreshToken)
		if time.Now().Before(needRefreshTime) {
			// no need refresh
			continue
		}

		// do refresh
		logger.Logf(true, "%s need to update GitHub token, will be update", target.UUID)
		installationID, err := gh.IsInstalledGitHubApp(ctx, target.GHEDomain.String, target.Scope)
		if err != nil {
			logger.Logf(false, "failed to get installationID: %+v", err)
			continue
		}
		clientApps, err := gh.NewClientGitHubApps(target.GHEDomain.String, config.Config.GitHub.AppID, config.Config.GitHub.PEMByte)
		if err != nil {
			logger.Logf(false, "failed to create client of GitHub installation: %+v", err)
			continue
		}
		token, expiredAt, err := gh.GenerateGitHubAppsToken(ctx, clientApps, installationID, target.Scope)
		if err != nil {
			logger.Logf(false, "failed to get Apps Token: %+v", err)
			continue
		}

		if err := m.ds.UpdateToken(ctx, target.UUID, token, *expiredAt); err != nil {
			logger.Logf(false, "failed to update token (target: %s): %+v", target.UUID, err)
			if err := datastore.UpdateTargetStatus(ctx, m.ds, target.UUID, datastore.TargetStatusErr, "can not update token"); err != nil {
				logger.Logf(false, "failed to update target status (target ID: %s): %+v\n", target.UUID, err)
			}
			continue
		}
	}

	return nil
}
