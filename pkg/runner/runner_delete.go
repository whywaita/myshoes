package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
	"github.com/whywaita/myshoes/pkg/shoes"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Runner is a runner implement
type Runner struct {
	status string
	github *github.Runner
	ds     *datastore.Runner
}

var (
	// ConcurrencyDeleting is value of concurrency
	ConcurrencyDeleting atomic.Int64
)

func (m *Manager) do(ctx context.Context) error {
	logger.Logf(true, "start runner manager")

	targets, err := datastore.ListTargets(ctx, m.ds)
	if err != nil {
		return fmt.Errorf("failed to get targets: %w", err)
	}

	logger.Logf(true, "found %d targets in datastore", len(targets))
	for _, target := range targets {
		logger.Logf(true, "start to search runner in %s", target.Scope)
		if err := m.removeRunners(ctx, target); err != nil {
			logger.Logf(false, "failed to delete runners (target: %s): %+v", target.Scope, err)
		}
	}

	return nil
}

func (m *Manager) removeRunners(ctx context.Context, t datastore.Target) error {
	runners, err := m.ds.ListRunnersByTargetID(ctx, t.UUID)
	if err != nil {
		return fmt.Errorf("failed to retrieve list of running runner: %w", err)
	}

	var mode TemporaryMode
	if strings.EqualFold(m.runnerVersion, "latest") {
		mode = TemporaryEphemeral
	} else {
		_, m, err := GetRunnerTemporaryMode(m.runnerVersion)
		if err != nil {
			return fmt.Errorf("failed to get runner mode: %w", err)
		}
		mode = m
	}

	ghRunners, err := isRegisteredRunnerZeroInGitHub(ctx, t)
	if err != nil {
		return fmt.Errorf("failed to check number of registerd runner: %w", err)
	}

	if len(ghRunners) == 0 && len(runners) == 0 {
		switch mode {
		case TemporaryOnce:
			logger.Logf(false, "runner for queueing is not found in %s", t.Scope)
			if err := datastore.UpdateTargetStatus(ctx, m.ds, t.UUID, datastore.TargetStatusErr, ErrDescriptionRunnerForQueueingIsNotFound); err != nil {
				logger.Logf(false, "failed to update target status (target ID: %s): %+v\n", t.UUID, err)
			}
		default:
			if t.Status == datastore.TargetStatusErr && t.StatusDescription.Valid && strings.EqualFold(t.StatusDescription.String, ErrDescriptionRunnerForQueueingIsNotFound) {
				if err := datastore.UpdateTargetStatus(ctx, m.ds, t.UUID, datastore.TargetStatusActive, ""); err != nil {
					logger.Logf(false, "failed to update target status (target ID: %s): %+v\n", t.UUID, err)
				}
			}
		}

		return nil
	}

	sem := semaphore.NewWeighted(config.Config.MaxConcurrencyDeleting)
	var eg errgroup.Group
	ConcurrencyDeleting.Store(0)

	for _, runner := range runners {
		runner := runner

		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to Acquire: %w", err)
		}
		ConcurrencyDeleting.Add(1)

		eg.Go(func() error {
			defer func() {
				sem.Release(1)
				ConcurrencyDeleting.Add(-1)
			}()

			if err := m.removeRunner(ctx, t, runner, ghRunners); err != nil {
				logger.Logf(false, "failed to delete runner: %+v", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to wait errgroup.Wait(): %w", err)
	}

	if t.Status == datastore.TargetStatusRunning {
		if err := datastore.UpdateTargetStatus(ctx, m.ds, t.UUID, datastore.TargetStatusActive, ""); err != nil {
			logger.Logf(false, "failed to update target status (target ID: %s): %+v\n", t.UUID, err)
		}
	}

	return nil
}

func (m *Manager) removeRunner(ctx context.Context, t datastore.Target, runner datastore.Runner, ghRunners []*github.Runner) error {
	if err := sanitizeRunnerMustRunningTime(runner); errors.Is(err, ErrNotWillDeleteRunner) {
		logger.Logf(false, "%s is not running MustRunningTime", runner.UUID)
		return nil
	}
	var mode TemporaryMode
	if strings.EqualFold(m.runnerVersion, "latest") {
		mode = TemporaryEphemeral
	} else {
		_, m, err := GetRunnerTemporaryMode(m.runnerVersion)
		if err != nil {
			return fmt.Errorf("failed to get runner mode: %w", err)
		}
		mode = m
	}

	switch mode {
	case TemporaryOnce:
		if err := m.removeRunnerModeOnce(ctx, t, runner, ghRunners); err != nil {
			return fmt.Errorf("failed to remove runner (mode once): %w", err)
		}
	case TemporaryEphemeral:
		if err := m.removeRunnerModeEphemeral(ctx, t, runner, ghRunners); err != nil {
			return fmt.Errorf("failed to remove runner (mode ephemeral): %w", err)
		}
	}

	return nil
}

func isRegisteredRunnerZeroInGitHub(ctx context.Context, t datastore.Target) ([]*github.Runner, error) {
	owner, repo := t.OwnerRepo()
	client, err := gh.NewClient(t.GitHubToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create github client: %w", err)
	}

	ghRunners, err := gh.ListRunners(ctx, client, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of runner in GitHub: %w", err)
	}

	return ghRunners, nil
}

var (
	// ErrNotWillDeleteRunner is error message for "not will delete runner"
	ErrNotWillDeleteRunner = fmt.Errorf("not will delete runner")
)

const (
	// ErrDescriptionRunnerForQueueingIsNotFound is error message for datastore.StatusDescription "runner for queueing is not found"
	ErrDescriptionRunnerForQueueingIsNotFound = "runner for queueing is not found"
)

var (
	// StatusWillDelete will delete target in GitHub runners
	StatusWillDelete = "offline"
	// StatusSleep is sleeping runners
	StatusSleep = "online"
)

func sanitizeGitHubRunner(ghRunner github.Runner, dsRunner datastore.Runner) error {
	if ghRunner.GetBusy() {
		// runner is busy, so not will delete
		return ErrNotWillDeleteRunner
	}

	switch ghRunner.GetStatus() {
	case StatusWillDelete:
		if err := sanitizeRunner(dsRunner, MustRunningTime); err != nil {
			logger.Logf(false, "%s is offline and not running %s, so not will delete (created_at: %s, now: %s)", dsRunner.UUID, MustRunningTime, dsRunner.CreatedAt, time.Now().UTC())
			return fmt.Errorf("failed to sanitize will delete runner: %w", err)
		}
		return nil
	case StatusSleep:
		if err := sanitizeRunner(dsRunner, MustGoalTime); err != nil {
			logger.Logf(false, "%s is idle and not running %s, so not will delete (created_at: %s, now: %s)", dsRunner.UUID, MustGoalTime, dsRunner.CreatedAt, time.Now().UTC())
			return fmt.Errorf("failed to sanitize idle runner: %w", err)
		}
		return nil
	}

	return ErrNotWillDeleteRunner
}

func sanitizeRunnerMustRunningTime(runner datastore.Runner) error {
	return sanitizeRunner(runner, MustRunningTime)
}

func sanitizeRunner(runner datastore.Runner, needTime time.Duration) error {
	spent := runner.CreatedAt.Add(needTime)
	now := time.Now().UTC()
	if !spent.Before(now) {
		return ErrNotWillDeleteRunner
	}

	return nil
}

// deleteRunnerWithGitHub delete runner in github, shoes, datastore.
// runnerUUID is uuid in datastore, runnerID is id from GitHub.
func (m *Manager) deleteRunnerWithGitHub(ctx context.Context, githubClient *github.Client, runner datastore.Runner, runnerID int64, owner, repo, runnerStatus string) error {
	logger.Logf(false, "will delete runner with GitHub: %s", runner.UUID.String())
	isOrg := false
	if repo == "" {
		isOrg = true
	}

	if isOrg {
		if _, err := githubClient.Actions.RemoveOrganizationRunner(ctx, owner, runnerID); err != nil {
			return fmt.Errorf("failed to remove organization runner (runner uuid: %s): %+v", runner.UUID.String(), err)
		}
	} else {
		if _, err := githubClient.Actions.RemoveRunner(ctx, owner, repo, runnerID); err != nil {
			return fmt.Errorf("failed to remove repository runner (runner uuid: %s): %+v", runner.UUID.String(), err)
		}
	}

	if err := m.deleteRunner(ctx, runner, runnerStatus); err != nil {
		return fmt.Errorf("failed to delete runner: %w", err)
	}
	return nil
}

// deleteRunner delete runner in shoes, datastore.
func (m *Manager) deleteRunner(ctx context.Context, runner datastore.Runner, runnerStatus string) error {
	logger.Logf(false, "will delete runner: %s", runner.UUID.String())

	client, teardown, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	labels, err := gh.ExtractRunsOnLabels([]byte(runner.RequestWebhook))
	if err != nil {
		return fmt.Errorf("failed to extract labels: %w", err)
	}

	if err := client.DeleteInstance(ctx, runner.CloudID, labels); err != nil {
		if status.Code(errors.Unwrap(err)) == codes.NotFound {
			logger.Logf(true, "%s is not found, will ignore from shoes", runner.UUID)
		} else {
			return fmt.Errorf("failed to delete instance: %w", err)
		}
	}

	now := time.Now().UTC()
	if err := m.ds.DeleteRunner(ctx, runner.UUID, now, ToReason(runnerStatus)); err != nil {
		return fmt.Errorf("failed to remove runner from datastore (runner uuid: %s): %+v", runner.UUID.String(), err)
	}

	return nil
}
