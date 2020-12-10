package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/whywaita/myshoes/pkg/shoes"

	"github.com/google/go-github/v32/github"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
)

var (
	GoalCheckerInterval = 1 * time.Minute
	MustRunningTime     = 10 * time.Minute // set time of instance create + download binaries + etc
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
				logger.Logf("failed to starter: %+v", err)
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

var (
	StatusWillDelete = "offline"
)

type Runner struct {
	github *github.Runner
	ds     *datastore.Runner
}

func (m *Manager) removeOfflineRunner(ctx context.Context, t *datastore.Target) error {
	var owner, repo string

	switch gh.DetectScope(t.Scope) {
	case gh.Organization:
		owner = t.Scope
		repo = ""
	case gh.Repository:
		s := strings.Split(t.Scope, "/")
		owner = s[0]
		repo = s[1]
	}

	client, err := gh.NewClient(ctx, t.GitHubPersonalToken, t.GHEDomain.String)
	if err != nil {
		return fmt.Errorf("failed to create github client: %w", err)
	}

	offlineRunners, err := getOfflineRunner(ctx, client, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get offline runner: %w", err)
	}

	sanitizedRunners, err := m.sanitizeOfflineRunner(ctx, offlineRunners)
	if err != nil {
		return fmt.Errorf("failed to sanitize offline runner: %w", err)
	}

	for _, offlineRunner := range sanitizedRunners {
		// delete runner from GitHub
		if err := m.deleteRunner(ctx, client, offlineRunner.ds, *offlineRunner.github.ID, owner, repo); err != nil {
			logger.Logf("failed to delete runner: %+v\n", err)
			continue
		}
	}

	return nil
}

func (m *Manager) sanitizeOfflineRunner(ctx context.Context, offlineRunners []*github.Runner) ([]Runner, error) {
	var sanitized []Runner

	if len(offlineRunners) == 1 {
		// only one runner for queuing, not remove.
		return nil, nil
	}

	for _, r := range offlineRunners {
		runnerUUID, err := ToUUID(*r.Name)
		if err != nil {
			//logger.Logf("not uuid in offline runner (runner name: %s), will ignore: %+v", *r.Name, err)
			continue
		}
		dsRunner, err := m.ds.GetRunner(ctx, runnerUUID)
		if err != nil {
			if err == datastore.ErrNotFound {
				// not managed in myshoes (maybe first runner)
			} else {
				logger.Logf("failed to retrieve repository runner (runner uuid: %s): %+v", runnerUUID, err)
			}

			continue
		}

		// not delete recently within MustRunningTime
		// this check protect to delete not running instance yet
		spent := dsRunner.CreatedAt.Add(MustRunningTime)
		if !spent.Before(time.Now()) {
			continue
		}

		runner := Runner{
			github: r,
			ds:     dsRunner,
		}
		sanitized = append(sanitized, runner)
	}

	return sanitized, nil
}

// getOfflineRunner retrieve runner that status is offline from GitHub.
func getOfflineRunner(ctx context.Context, githubClient *github.Client, owner, repo string) ([]*github.Runner, error) {
	var rs []*github.Runner
	var offlineRunners []*github.Runner

	isOrg := false
	if repo == "" {
		isOrg = true
	}

	var opts = &github.ListOptions{
		Page:    0,
		PerPage: 10,
	}

	for {
		var runners *github.Runners
		var err error

		if isOrg {
			runners, _, err = githubClient.Actions.ListOrganizationRunners(ctx, owner, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list organization runners: %w", err)
			}
		} else {
			runners, _, err = githubClient.Actions.ListRunners(ctx, owner, repo, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list repository runners: %w", err)
			}
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

	return offlineRunners, nil
}

// deleteRunner delete runner in github, shoes, datastore.
// runnerUUID is uuid in datastore, runnerID is id from GitHub.
func (m *Manager) deleteRunner(ctx context.Context, githubClient *github.Client, runner *datastore.Runner, runnerID int64, owner, repo string) error {
	logger.Logf("will delete repository runner: %s", runner.UUID.String())

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

	client, teardown, err := shoes.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	if err := client.DeleteInstance(ctx, runner.CloudID); err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	if err := m.ds.DeleteRunner(ctx, runner.UUID, time.Now()); err != nil {
		return fmt.Errorf("failed to remove runner from datastore (runner uuid: %s): %+v", runner.UUID.String(), err)
	}

	return nil
}

func ToName(uuid string) string {
	return fmt.Sprintf("myshoes-%s", uuid)
}

func ToUUID(name string) (uuid.UUID, error) {
	u := strings.TrimPrefix(name, "myshoes-")
	return uuid.FromString(u)
}
