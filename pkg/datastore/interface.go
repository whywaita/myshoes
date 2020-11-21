package datastore

import (
	"database/sql"
	"errors"
	"time"
)

// Error values
var (
	ErrNotFound = errors.New("not found")
)

// Datastore is persistent storage
type Datastore interface {
	CreateTarget(target Target) error
	GetTarget(uuid string) (*Target, error)
	GetTargetByScope(gheDomain, scope string) (*Target, error)
	DeleteTarget(uuid string) error
}

// Target is a target that will add auto-scaling runner.
type Target struct {
	UUID                string         `db:"uuid" json:"id"`
	Scope               string         `db:"scope" json:"scope"`                                 // repo (:owner/:repo) or org (:organization)
	GitHubPersonalToken string         `db:"github_personal_token" json:"github_personal_token"` // TODO: encrypt
	GHEDomain           sql.NullString `db:"ghe_domain" json:"ghe_domain"`
	CreatedAt           time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time      `db:"updated_at" json:"updated_at"`
}
