package datastore

import (
	"context"
	"database/sql"
	"errors"
	"path"
	"time"

	uuid "github.com/satori/go.uuid"
)

// Error values
var (
	ErrNotFound = errors.New("not found")
)

// Datastore is persistent storage
type Datastore interface {
	CreateTarget(ctx context.Context, target Target) error
	GetTarget(ctx context.Context, id uuid.UUID) (*Target, error)
	GetTargetByScope(ctx context.Context, gheDomain, scope string) (*Target, error)
	ListTargets(ctx context.Context) ([]Target, error)
	DeleteTarget(ctx context.Context, id uuid.UUID) error

	UpdateStatus(ctx context.Context, targetID uuid.UUID, newStatus Status, description string) error

	EnqueueJob(ctx context.Context, job Job) error
	ListJobs(ctx context.Context) ([]Job, error)
	DeleteJob(ctx context.Context, id uuid.UUID) error

	CreateRunner(ctx context.Context, runner Runner) error
	ListRunners(ctx context.Context) ([]Runner, error)
	GetRunner(ctx context.Context, id uuid.UUID) (*Runner, error)
	DeleteRunner(ctx context.Context, id uuid.UUID, deletedAt time.Time) error
}

// Target is a target repository that will add auto-scaling runner.
type Target struct {
	UUID                uuid.UUID      `db:"uuid" json:"id"`
	Scope               string         `db:"scope" json:"scope"`                                 // repo (:owner/:repo) or org (:organization)
	GitHubPersonalToken string         `db:"github_personal_token" json:"github_personal_token"` // TODO: encrypt
	GHEDomain           sql.NullString `db:"ghe_domain" json:"ghe_domain"`
	ResourceType        ResourceType   `db:"resource_type" json:"resource_type"`
	RunnerUser          sql.NullString `db:"runner_user" json:"runner_user"`
	Status              Status         `db:"status" json:"status"`
	StatusDescription   sql.NullString `db:"status_description" json:"status_description"`
	CreatedAt           time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time      `db:"updated_at" json:"updated_at"`
}

// RepoURL return repository URL.
func (t *Target) RepoURL() string {
	serverURL := "https://github.com"
	if t.GHEDomain.Valid {
		serverURL = t.GHEDomain.String
	}

	return path.Join(serverURL, t.Scope)
}

// ResourceType is runner machine spec
type ResourceType string

// ResourceTypes variables
const (
	Nano    ResourceType = "nano"
	Micro                = "micro"
	Small                = "small"
	Medium               = "medium"
	Large                = "large"
	XLarge               = "xlarge"
	XLarge2              = "2xlarge"
	XLarge3              = "3xlarge"
	XLarge4              = "4xlarge"
)

// Status is status for target
type Status string

// Status variables
const (
	TargetStatusActive  Status = "active"
	TargetStatusRunning        = "running"
	TargetStatusErr            = "error"
)

func (r *ResourceType) String() string {
	return string(*r)
}

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

// Runner is a runner
type Runner struct {
	UUID      uuid.UUID    `db:"uuid"`
	ShoesType string       `db:"shoes_type"`
	IPAddress string       `db:"ip_address"`
	TargetID  uuid.UUID    `db:"target_id"`
	CloudID   string       `db:"cloud_id"`
	Deleted   bool         `db:"deleted"`
	CreatedAt time.Time    `db:"created_at"`
	UpdatedAt time.Time    `db:"updated_at"`
	DeletedAt sql.NullTime `db:"deleted_at"`
}
