package datastore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/go-github/v32/github"
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
	DeleteTarget(ctx context.Context, id uuid.UUID) error

	EnqueueJob(ctx context.Context, job Job) error
	GetJob(ctx context.Context) ([]Job, error)
	DeleteJob(ctx context.Context, id uuid.UUID) error
}

// Target is a target repository that will add auto-scaling runner.
type Target struct {
	UUID                uuid.UUID      `db:"uuid" json:"id"`
	Scope               string         `db:"scope" json:"scope"`                                 // repo (:owner/:repo) or org (:organization)
	GitHubPersonalToken string         `db:"github_personal_token" json:"github_personal_token"` // TODO: encrypt
	GHEDomain           sql.NullString `db:"ghe_domain" json:"ghe_domain"`
	ResourceType        ResourceType   `db:"resource_type" json:"resource_type"`
	CreatedAt           time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time      `db:"updated_at" json:"updated_at"`
}

// ResourceType is runner machine spec
type ResourceType string

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

func (r *ResourceType) String() string {
	return string(*r)
}

// Job is a runner job
type Job struct {
	UUID           uuid.UUID            `db:"uuid"`
	GHEDomain      sql.NullString       `db:"ghe_domain"`
	Repository     string               `db:"repository"`
	CheckEventJSON github.CheckRunEvent `db:"check_event"`
	CreatedAt      time.Time            `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time            `db:"updated_at" json:"updated_at"`
}
