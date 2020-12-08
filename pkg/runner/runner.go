package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
)

var (
	GoalCheckerInterval = 10 * time.Second
)

type Manager struct {
	ds datastore.Datastore
}

func New(ds datastore.Datastore) *Manager {
	return &Manager{
		ds: ds,
	}
}

// Loop check
func (m *Manager) Loop(ctx context.Context) error {
	logger.Logf("start runner loop")

	ticker := time.NewTicker(GoalCheckerInterval)

	for {
		select {
		case <-ticker.C:
			if err := m.do(ctx); err != nil {
				logger.Logf("%+v", err)
			}
		}
	}
}

func (m *Manager) do(ctx context.Context) error {
	runners, err := m.ds.ListRunners(ctx)
	if err != nil {
		return fmt.Errorf("failed to get runners: %w", err)
	}

	for _, runner := range runners {
		t, err := m.ds.GetTarget(ctx, runner.TargetID)
		if err != nil {
			return fmt.Errorf("failed to retrieve target: %w", err)
		}

		if err := m.removeOfflineRunner(ctx, t); err != nil {
			return fmt.Errorf("failed to retrieve runners: %w", err)
		}
	}

	return nil
}

func (m *Manager) removeOfflineRunner(ctx context.Context, t *datastore.Target) error {
	switch gh.DetectScope(t.Scope) {
	case gh.Repository:
		return m.removeOfflineRunnerRepo(ctx, t)
	case gh.Organization:
		return m.removeOfflineRunnerOrg(ctx, t)
	default:
		return fmt.Errorf("failed to detect scope")
	}
}

var (
	StatusWillDelete = "offline"
)

func (m *Manager) removeOfflineRunnerOrg(ctx context.Context, t *datastore.Target) error {
	var rs []*github.Runner
	var offlineRunners []*github.Runner
	client := gh.NewClient(ctx, t.GitHubPersonalToken)

	var opts = &github.ListOptions{
		Page:    0,
		PerPage: 10,
	}

	for {
		runners, _, err := client.Actions.ListOrganizationRunners(ctx, t.Scope, opts)
		if err != nil {
			return fmt.Errorf("failed to list organization runners: %w", err)
		}

		for _, r := range runners.Runners {
			if r.GetStatus() == StatusWillDelete {
				offlineRunners = append(offlineRunners, r)
			}
		}

		rs = append(rs, runners.Runners...)
		if len(rs) == runners.TotalCount {
			break
		}
		opts.Page = opts.Page + 1
	}

	if len(offlineRunners) == 1 {
		// only one runner for queuing, not remove.
		return nil
	}

	for _, offlineRunner := range offlineRunners {
		runnerUUID, err := ToUUID(*offlineRunner.Name)
		if err != nil {
			logger.Logf("not uuid in offline runner (runner name: %s), will ignore: %+v", *offlineRunner.Name, err)
			continue
		}
		if _, err := m.ds.GetRunner(ctx, runnerUUID); err != nil {
			if err == datastore.ErrNotFound {
				// not managed in myshoes (maybe first runner)
			} else {
				logger.Logf("failed to retrieve organization runner (runner uuid: %s)  %+v", runnerUUID, err)
			}

			continue
		}

		// delete runner from GitHub
		logger.Logf("will delete organization runner: %s", runnerUUID)
		if _, err := client.Actions.RemoveOrganizationRunner(ctx, t.Scope, *offlineRunner.ID); err != nil {
			logger.Logf("failed to remove organization runner (runner uuid: %s): %+v", runnerUUID, err)
			continue
		}
		if err := m.ds.DeleteRunner(ctx, runnerUUID, time.Now()); err != nil {
			logger.Logf("failed to remove organization runner from datastore (runner uuid: %s): %+v", runnerUUID, err)
			continue
		}
	}

	return nil
}

func (m *Manager) removeOfflineRunnerRepo(ctx context.Context, t *datastore.Target) error {
	s := strings.Split(t.Scope, "/")
	owner := s[0]
	repo := s[1]

	var rs []*github.Runner
	var offlineRunners []*github.Runner
	client := gh.NewClient(ctx, t.GitHubPersonalToken)

	var opts = &github.ListOptions{
		Page:    0,
		PerPage: 10,
	}

	for {
		runners, _, err := client.Actions.ListRunners(ctx, owner, repo, opts)
		if err != nil {
			return fmt.Errorf("failed to list repository runners: %w", err)
		}

		for _, r := range runners.Runners {
			if r.GetStatus() == StatusWillDelete {
				offlineRunners = append(offlineRunners, r)
			}
		}

		rs = append(rs, runners.Runners...)
		if len(rs) == runners.TotalCount {
			break
		}
		opts.Page = opts.Page + 1
	}

	if len(offlineRunners) == 1 {
		// only one runner for queuing, not remove.
		return nil
	}

	for _, offlineRunner := range offlineRunners {
		runnerUUID, err := ToUUID(*offlineRunner.Name)
		if err != nil {
			logger.Logf("not uuid in offline runner (runner name: %s), will ignore: %+v", *offlineRunner.Name, err)
			continue
		}
		if _, err := m.ds.GetRunner(ctx, runnerUUID); err != nil {
			if err == datastore.ErrNotFound {
				// not managed in myshoes (maybe first runner)
			} else {
				logger.Logf("failed to retrieve repository runner (runner uuid: %s): %+v", runnerUUID, err)
			}

			continue
		}

		// delete runner from GitHub
		logger.Logf("will delete repository runner: %s", runnerUUID)
		if _, err := client.Actions.RemoveRunner(ctx, owner, repo, *offlineRunner.ID); err != nil {
			logger.Logf("failed to remove repository runner (runner uuid): %+v", t.UUID, err)
			continue
		}
		if err := m.ds.DeleteRunner(ctx, runnerUUID, time.Now()); err != nil {
			logger.Logf("failed to remove repository runner from datastore (runner uuid: %s): %+v", runnerUUID, err)
			continue
		}
	}

	return nil
}

func ToName(uuid string) string {
	return fmt.Sprintf("myshoes-%s", uuid)
}

func ToUUID(name string) (uuid.UUID, error) {
	u := strings.TrimLeft(name, "myshoes-")
	return uuid.FromString(u)
}
