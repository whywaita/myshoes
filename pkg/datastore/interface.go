package datastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-version"

	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
)

// Error values
var (
	ErrNotFound = errors.New("not found")
)

// Lock values
var (
	IsLocked    = "is locked"
	IsNotLocked = "is not locked"
)

// Datastore is persistent storage
type Datastore interface {
	CreateTarget(ctx context.Context, target Target) error
	GetTarget(ctx context.Context, id uuid.UUID) (*Target, error)
	GetTargetByScope(ctx context.Context, gheDomain, scope string) (*Target, error)
	ListTargets(ctx context.Context) ([]Target, error)
	DeleteTarget(ctx context.Context, id uuid.UUID) error

	// Deprecated: Use datastore.UpdateTargetStatus.
	UpdateTargetStatus(ctx context.Context, targetID uuid.UUID, newStatus TargetStatus, description string) error
	UpdateToken(ctx context.Context, targetID uuid.UUID, newToken string, newExpiredAt time.Time) error

	UpdateTargetParam(ctx context.Context, targetID uuid.UUID, newResourceType ResourceType, newRunnerVersion, newRunnerUser, newProviderURL string) error

	EnqueueJob(ctx context.Context, job Job) error
	ListJobs(ctx context.Context) ([]Job, error)
	DeleteJob(ctx context.Context, id uuid.UUID) error

	CreateRunner(ctx context.Context, runner Runner) error
	ListRunners(ctx context.Context) ([]Runner, error)
	GetRunner(ctx context.Context, id uuid.UUID) (*Runner, error)
	DeleteRunner(ctx context.Context, id uuid.UUID, deletedAt time.Time, reason RunnerStatus) error

	// Lock
	GetLock(ctx context.Context) error
	IsLocked(ctx context.Context) (string, error)
}

// Target is a target repository that will add auto-scaling runner.
type Target struct {
	UUID              uuid.UUID      `db:"uuid" json:"id"`
	Scope             string         `db:"scope" json:"scope"` // repo (:owner/:repo) or org (:organization)
	GitHubToken       string         `db:"github_token" json:"github_token"`
	TokenExpiredAt    time.Time      `db:"token_expired_at" json:"token_expired_at"`
	GHEDomain         sql.NullString `db:"ghe_domain" json:"ghe_domain"`
	ResourceType      ResourceType   `db:"resource_type" json:"resource_type"`
	RunnerUser        sql.NullString `db:"runner_user" json:"runner_user"`
	RunnerVersion     sql.NullString `db:"runner_version" json:"runner_version"`
	ProviderURL       sql.NullString `db:"provider_url" json:"provider_url"`
	Status            TargetStatus   `db:"status" json:"status"`
	StatusDescription sql.NullString `db:"status_description" json:"status_description"`
	CreatedAt         time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at" json:"updated_at"`
}

// RepoURL return repository URL.
func (t *Target) RepoURL() string {
	serverURL := "https://github.com"
	if t.GHEDomain.Valid {
		serverURL = t.GHEDomain.String
	}

	s := strings.Split(serverURL, "://")

	var u url.URL
	u.Scheme = s[0]
	u.Host = s[1]
	u.Path = t.Scope

	return u.String()
}

// OwnerRepo return :owner and :repo
func (t *Target) OwnerRepo() (string, string) {
	return gh.DivideScope(t.Scope)
}

// CanReceiveJob check status in target
func (t *Target) CanReceiveJob() bool {
	switch t.Status {
	case TargetStatusSuspend, TargetStatusDeleted:
		return false
	}

	return true
}

// ListTargets get list of target that can receive job
func ListTargets(ctx context.Context, ds Datastore) ([]Target, error) {
	targets, err := ds.ListTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get targets from datastore: %w", err)
	}

	var result []Target

	for _, t := range targets {
		if t.CanReceiveJob() {
			result = append(result, t)
		}
	}

	return result, nil
}

// UpdateTargetStatus update datastore
func UpdateTargetStatus(ctx context.Context, ds Datastore, targetID uuid.UUID, newStatus TargetStatus, description string) error {
	target, err := ds.GetTarget(ctx, targetID)
	if err != nil {
		return fmt.Errorf("failed to get target: %w", err)
	}

	if !target.CanReceiveJob() {
		// not change status
		return nil
	}

	if err := ds.UpdateTargetStatus(ctx, targetID, newStatus, description); err != nil {
		logger.Logf(false, "failed to update target status: %+v", err)
		return err
	}

	return nil
}

// TargetStatus is status for target
type TargetStatus string

// TargetStatus variables
const (
	TargetStatusActive  TargetStatus = "active"
	TargetStatusRunning              = "running"
	TargetStatusSuspend              = "suspend"
	TargetStatusDeleted              = "deleted"
	TargetStatusErr                  = "error"
)

// Job is a runner job
type Job struct {
	UUID           uuid.UUID      `db:"uuid"`
	GHEDomain      sql.NullString `db:"ghe_domain"`
	Repository     string         `db:"repository"` // repo (:owner/:repo)
	CheckEventJSON string         `db:"check_event"`
	TargetID       uuid.UUID      `db:"target_id"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updated_at"`
}

// RepoURL return repository URL that send webhook.
func (j *Job) RepoURL() string {
	serverURL := "https://github.com"
	if j.GHEDomain.Valid {
		serverURL = j.GHEDomain.String
	}

	s := strings.Split(serverURL, "://")

	var u url.URL
	u.Scheme = s[0]
	u.Host = s[1]
	u.Path = j.Repository

	return u.String()
}

// Runner is a runner
type Runner struct {
	UUID           uuid.UUID      `db:"runner_id"`
	ShoesType      string         `db:"shoes_type"`
	IPAddress      string         `db:"ip_address"`
	TargetID       uuid.UUID      `db:"target_id"`
	CloudID        string         `db:"cloud_id"`
	Deleted        bool           `db:"deleted"`
	Status         RunnerStatus   `db:"status"`
	ResourceType   ResourceType   `db:"resource_type"`
	RunnerUser     sql.NullString `db:"runner_user" json:"runner_user"`
	RunnerVersion  sql.NullString `db:"runner_version" json:"runner_version"`
	ProviderURL    sql.NullString `db:"provider_url" json:"provider_url"`
	RepositoryURL  string         `db:"repository_url"`
	RequestWebhook string         `db:"request_webhook"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
	DeletedAt      sql.NullTime   `db:"deleted_at"`
}

// RunnerStatus is status for runner
type RunnerStatus string

// RunnerStatus variables
const (
	RunnerStatusCreated        RunnerStatus = "created"
	RunnerStatusCompleted                   = "completed"
	RunnerStatusReachHardLimit              = "reach_hard_limit"
)

const (
	// DefaultRunnerVersion is default value of actions/runner
	DefaultRunnerVersion = "v2.275.1"
)

// RunnerTemporaryMode is mode of temporary runner
type RunnerTemporaryMode int

// RunnerEphemeralModes variable
const (
	RunnerTemporaryUnknown RunnerTemporaryMode = iota
	RunnerTemporaryOnce
	RunnerTemporaryEphemeral
)

// StringFlag return flag
func (rtm RunnerTemporaryMode) StringFlag() string {
	switch rtm {
	case RunnerTemporaryOnce:
		return "--once"
	case RunnerTemporaryEphemeral:
		return "--ephemeral"
	}
	return "unknown"
}

// GetRunnerTemporaryMode get runner version and RunnerTemporaryMode
func GetRunnerTemporaryMode(runnerVersion sql.NullString) (string, RunnerTemporaryMode, error) {
	if !runnerVersion.Valid {
		// not set, return default
		return DefaultRunnerVersion, RunnerTemporaryOnce, nil
	}

	ephemeralSupportVersion, err := version.NewVersion("v2.282.0")
	if err != nil {
		return "", RunnerTemporaryUnknown, fmt.Errorf("failed to parse ephemeral runner version: %w", err)
	}

	inputVersion, err := version.NewVersion(runnerVersion.String)
	if err != nil {
		return "", RunnerTemporaryUnknown, fmt.Errorf("failed to parse input runner version: %w", err)
	}

	if ephemeralSupportVersion.GreaterThan(inputVersion) {
		return runnerVersion.String, RunnerTemporaryOnce, nil
	}
	return runnerVersion.String, RunnerTemporaryEphemeral, nil
}
