package runner

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-github/v35/github"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
)

// removeRunnerModeOnce remove runner that created by --once flag.
// --once flag is not delete self-hosted runner when end of job. So, The origin list of runner from GitHub.
func (m *Manager) removeRunnerModeOnce(ctx context.Context, t datastore.Target, runner datastore.Runner, ghRunners []*github.Runner) error {
	owner, repo := t.OwnerRepo()
	client, err := gh.NewClient(t.GitHubToken, t.GHEDomain.String)
	if err != nil {
		return fmt.Errorf("failed to create github client: %w", err)
	}

	ghRunner, err := gh.ExistGitHubRunnerWithRunner(ghRunners, ToName(runner.UUID.String()))
	switch {
	case errors.Is(err, gh.ErrNotFound):
		logger.Logf(false, "NotFound in GitHub, so will delete in datastore without GitHub (runner: %s)", runner.UUID.String())
		if err := m.deleteRunner(ctx, runner, StatusWillDelete); err != nil {
			if err := datastore.UpdateTargetStatus(ctx, m.ds, t.UUID, datastore.TargetStatusErr, ""); err != nil {
				logger.Logf(false, "failed to update target status (target ID: %s): %+v\n", t.UUID, err)
			}

			return fmt.Errorf("failed to delete runner: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("failed to check runner exist in GitHub (runner: %s): %w", runner.UUID, err)
	}

	if err := sanitizeGitHubRunner(*ghRunner, runner); err != nil {
		if errors.Is(err, ErrNotWillDeleteRunner) {
			return nil
		}

		return fmt.Errorf("failed to check runner of status: %w", err)
	}

	if err := m.deleteRunnerWithGitHub(ctx, client, runner, ghRunner.GetID(), owner, repo, ghRunner.GetStatus()); err != nil {
		if err := datastore.UpdateTargetStatus(ctx, m.ds, t.UUID, datastore.TargetStatusErr, ""); err != nil {
			logger.Logf(false, "failed to update target status (target ID: %s): %+v\n", t.UUID, err)
		}

		return fmt.Errorf("failed to delete runner with GitHub: %w", err)
	}
	return nil
}
