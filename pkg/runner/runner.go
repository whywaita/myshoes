package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v35/github"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
	"github.com/whywaita/myshoes/pkg/shoes"
)

var (
	// GoalCheckerInterval is interval time of check deleting runner
	GoalCheckerInterval = 1 * time.Minute
	// MustGoalTime is hard limit for idle runner
	MustGoalTime = 1 * time.Hour
	// MustRunningTime is set time of instance create + download binaries + etc
	MustRunningTime = 5 * time.Minute
	// TargetTokenInterval is interval time of checking target token
	TargetTokenInterval = 5 * time.Minute
	//NeedRefreshToken is time of token expired
	NeedRefreshToken = 10 * time.Minute
)

// Manager is runner management
type Manager struct {
	ds datastore.Datastore
}

// New create a Manager
func New(ds datastore.Datastore) *Manager {
	return &Manager{
		ds: ds,
	}
}

// Loop check
func (m *Manager) Loop(ctx context.Context) error {
	logger.Logf(false, "start runner loop")

	ticker := time.NewTicker(GoalCheckerInterval)
	defer ticker.Stop()
	tokenRefreshTicker := time.NewTicker(TargetTokenInterval)
	defer tokenRefreshTicker.Stop()

	for {
		select {
		case <-tokenRefreshTicker.C:
			if err := m.doTargetToken(ctx); err != nil {
				logger.Logf(false, "failed to refresh token: %+v", err)
			}

		case <-ticker.C:
			if err := m.do(ctx); err != nil {
				logger.Logf(false, "failed to starter: %+v", err)
			}
		}
	}
}

func (m *Manager) do(ctx context.Context) error {
	logger.Logf(true, "start runner manager")

	targets, err := datastore.ListTargets(ctx, m.ds)
	if err != nil {
		return fmt.Errorf("failed to get targets: %w", err)
	}

	logger.Logf(true, "found %d targets in datastore", len(targets))
	for _, target := range targets {
		if err := m.removeRunner(ctx, &target); err != nil {
			return fmt.Errorf("failed to delete runners: %w", err)
		}
	}

	return nil
}

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
		token, expiredAt, err := gh.GenerateGitHubAppsToken(target.GHEDomain.String, installationID)
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

// Runner is a runner implement
type Runner struct {
	status string
	github *github.Runner
	ds     *datastore.Runner
}

func (m *Manager) removeRunner(ctx context.Context, t *datastore.Target) error {
	logger.Logf(true, "start to search runner in %s", t.RepoURL())

	owner, repo := t.OwnerRepo()
	client, err := gh.NewClient(ctx, t.GitHubToken, t.GHEDomain.String)
	if err != nil {
		return fmt.Errorf("failed to create github client: %w", err)
	}

	targetRunners, err := getDeleteTargetRunner(ctx, client, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get offline runner: %w", err)
	}
	logger.Logf(true, "found all delete target runners is %d in %s", len(targetRunners), t.RepoURL())

	if len(targetRunners) == 0 {
		logger.Logf(false, "runner for queueing is not found in %s", t.RepoURL())
		if err := datastore.UpdateTargetStatus(ctx, m.ds, t.UUID, datastore.TargetStatusErr, "runner for queueing is not found"); err != nil {
			logger.Logf(false, "failed to update target status (target ID: %s): %+v\n", t.UUID, err)
		}
		return nil
	}

	sanitizedRunners, err := m.sanitizeRunner(ctx, targetRunners)
	if err != nil {
		return fmt.Errorf("failed to sanitize offline runner: %w", err)
	}
	logger.Logf(true, "will be deleted %d offline runners in %s", len(sanitizedRunners), t.RepoURL())

	for _, r := range sanitizedRunners {
		if err := m.deleteRunner(ctx, client, r.ds, *r.github.ID, owner, repo, r.status); err != nil {
			logger.Logf(false, "failed to delete runner: %+v\n", err)

			if err := datastore.UpdateTargetStatus(ctx, m.ds, t.UUID, datastore.TargetStatusErr, ""); err != nil {
				logger.Logf(false, "failed to update target status (target ID: %s): %+v\n", t.UUID, err)
			}

			continue
		}
	}

	if err := datastore.UpdateTargetStatus(ctx, m.ds, t.UUID, datastore.TargetStatusActive, ""); err != nil {
		logger.Logf(false, "failed to update target status (target ID: %s): %+v\n", t.UUID, err)
		return fmt.Errorf("failed to update target status: %w", err)
	}

	return nil
}

func (m *Manager) sanitizeRunner(ctx context.Context, targetRunners []*github.Runner) ([]Runner, error) {
	var sanitized []Runner

	if len(targetRunners) == 1 {
		if _, err := ToUUID(*targetRunners[0].Name); err != nil {
			// targetRunner is not created by myshoes, so it's correct
			// only one runner for queuing, not remove.
			logger.Logf(true, "found only one delete target runner. maybe this runner set for queueing. not will delete.")
			return nil, nil
		}
	}

	for _, r := range targetRunners {
		runnerUUID, err := ToUUID(*r.Name)
		if err != nil {
			logger.Logf(true, "not uuid in target runner (runner name: %s), will ignore: %+v", *r.Name, err)
			continue
		}
		dsRunner, err := m.ds.GetRunner(ctx, runnerUUID)
		if err != nil {
			if err == datastore.ErrNotFound {
				logger.Logf(false, "runner name %s is the suitable format in myshoes, but not found in datastore", *r.Name)
			} else {
				logger.Logf(false, "failed to retrieve repository runner (runner uuid: %s): %+v", runnerUUID, err)
			}

			continue
		}

		switch r.GetStatus() {
		case StatusWillDelete:
			if err := sanitizeOfflineRunner(dsRunner); err != nil {
				continue
			}
		case StatusSleep:
			if err := sanitizeIdleRunner(dsRunner); err != nil {
				continue
			}
		}

		runner := Runner{
			status: r.GetStatus(),
			github: r,
			ds:     dsRunner,
		}
		sanitized = append(sanitized, runner)
	}

	return sanitized, nil
}

// Error values
var (
	ErrNotWillDeleteRunner = fmt.Errorf("not will delete runner")
)

// sanitizeOfflineRunner check runner running MustRunningTime.
func sanitizeOfflineRunner(r *datastore.Runner) error {
	// not delete recently within MustRunningTime
	// this check protect to delete not running instance yet
	spent := r.CreatedAt.Add(MustRunningTime)
	now := time.Now().UTC()
	if !spent.Before(now) {
		logger.Logf(false, "%s is not running %s, so not will delete (created_at: %s, now: %s)", r.UUID, MustRunningTime, r.CreatedAt, now)
		return ErrNotWillDeleteRunner
	}

	return nil
}

// sanitizeIdleRunner check no job runner between MustGoalTime.
func sanitizeIdleRunner(r *datastore.Runner) error {
	spent := r.CreatedAt.Add(MustGoalTime)
	now := time.Now().UTC()
	if !spent.Before(now) {
		logger.Logf(true, "%s is idle runner but not running in %s. so not wil delete (created_at: %s, now: %s)", r.UUID, MustGoalTime, r.CreatedAt, now)
		return ErrNotWillDeleteRunner
	}

	logger.Logf(false, "found %s is running between %s, will delete (created_at: %s, now: %s", r.UUID, MustGoalTime, r.CreatedAt, now)
	return nil
}

var (
	// StatusWillDelete will delete target in GitHub runners
	StatusWillDelete = "offline"
	// StatusSleep is sleeping runners
	StatusSleep = "online"
)

// getDeleteTargetRunner retrieve runner that status is offline or idle from GitHub.
func getDeleteTargetRunner(ctx context.Context, githubClient *github.Client, owner, repo string) ([]*github.Runner, error) {
	var runners []*github.Runner

	all, err := gh.ListRunners(ctx, githubClient, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get runner from GitHub: %w", err)
	}

	for _, r := range all {
		if r.GetStatus() == StatusWillDelete || r.GetStatus() == StatusSleep {
			runners = append(runners, r)
		}
	}
	return runners, nil
}

// deleteRunner delete runner in github, shoes, datastore.
// runnerUUID is uuid in datastore, runnerID is id from GitHub.
func (m *Manager) deleteRunner(ctx context.Context, githubClient *github.Client, runner *datastore.Runner, runnerID int64, owner, repo, runnerStatus string) error {
	logger.Logf(false, "will delete runner: %s", runner.UUID.String())

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

	now := time.Now().UTC()
	if err := m.ds.DeleteRunner(ctx, runner.UUID, now, ToReason(runnerStatus)); err != nil {
		return fmt.Errorf("failed to remove runner from datastore (runner uuid: %s): %+v", runner.UUID.String(), err)
	}

	return nil
}

// ToName convert uuid to runner name
func ToName(uuid string) string {
	return fmt.Sprintf("myshoes-%s", uuid)
}

// ToUUID convert runner name to uuid
func ToUUID(name string) (uuid.UUID, error) {
	u := strings.TrimPrefix(name, "myshoes-")
	return uuid.FromString(u)
}

// ToReason convert status from GitHub to datastore.RunnerStatus
func ToReason(status string) datastore.RunnerStatus {
	switch status {
	case StatusWillDelete:
		// is offline
		return datastore.RunnerStatusCompleted
	case StatusSleep:
		// is idle, reach hard limit
		return datastore.RunnerStatusReachHardLimit
	}

	return ""
}
