package datastore

import "time"

// Datastore is persistent storage
type Datastore interface {
	CreateTarget(target Target) error
	GetTarget(uuid string) (*Target, error)
	GetTargetByScope(scope string) (*Target, error)
	DeleteTarget(uuid string) error
}

// Target is a target that will add auto-scaling runner.
type Target struct {
	UUID                string    `db:"uuid"`
	Scope               string    `db:"scope"`                 // repo (:owner/:repo) or org (:organization)
	GitHubPersonalToken string    `db:"github_personal_token"` // TODO: encrypt
	CreatedAt           time.Time `db:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"`
}
