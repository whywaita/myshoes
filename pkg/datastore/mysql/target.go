package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/whywaita/myshoes/pkg/datastore"
	"uuid"
)

// rowTarget mirrors datastore.Target but stores the UUID column as a string so
// that database/sql can scan the VARCHAR(36) uuid column. The standard library
// uuid.UUID is a [16]byte with no sql.Scanner, so it cannot be scanned directly.
type rowTarget struct {
	UUID              string                 `db:"uuid"`
	Scope             string                 `db:"scope"`
	GitHubToken       string                 `db:"github_token"`
	TokenExpiredAt    time.Time              `db:"token_expired_at"`
	GHEDomain         sql.NullString         `db:"ghe_domain"`
	ResourceType      datastore.ResourceType `db:"resource_type"`
	ProviderURL       sql.NullString         `db:"provider_url"`
	Status            datastore.TargetStatus `db:"status"`
	StatusDescription sql.NullString         `db:"status_description"`
	CreatedAt         time.Time              `db:"created_at"`
	UpdatedAt         time.Time              `db:"updated_at"`
}

func (r rowTarget) target() (datastore.Target, error) {
	u, err := uuid.Parse(r.UUID)
	if err != nil {
		return datastore.Target{}, fmt.Errorf("failed to parse target uuid %q: %w", r.UUID, err)
	}

	return datastore.Target{
		UUID:              u,
		Scope:             r.Scope,
		GitHubToken:       r.GitHubToken,
		TokenExpiredAt:    r.TokenExpiredAt,
		GHEDomain:         r.GHEDomain,
		ResourceType:      r.ResourceType,
		ProviderURL:       r.ProviderURL,
		Status:            r.Status,
		StatusDescription: r.StatusDescription,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}, nil
}

func targetsFromRows(rows []rowTarget) ([]datastore.Target, error) {
	ts := make([]datastore.Target, 0, len(rows))
	for _, r := range rows {
		t, err := r.target()
		if err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return ts, nil
}

// CreateTarget create a target
func (m *MySQL) CreateTarget(ctx context.Context, target datastore.Target) error {
	expiredAtRFC3339 := target.TokenExpiredAt.Format("2006-01-02 15:04:05")

	query := `INSERT INTO targets(uuid, scope, ghe_domain, github_token, token_expired_at, resource_type, provider_url) VALUES (?, ?, ?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(
		ctx,
		query,
		target.UUID.String(),
		target.Scope,
		target.GHEDomain,
		target.GitHubToken,
		expiredAtRFC3339,
		target.ResourceType,
		target.ProviderURL,
	); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

// GetTarget get a target
func (m *MySQL) GetTarget(ctx context.Context, id uuid.UUID) (*datastore.Target, error) {
	var row rowTarget
	query := `SELECT uuid, scope, github_token, token_expired_at, resource_type, provider_url, status, status_description, created_at, updated_at FROM targets WHERE uuid = ?`
	if err := m.Conn.GetContext(ctx, &row, query, id.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	t, err := row.target()
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTargetByScope get a target from scope
func (m *MySQL) GetTargetByScope(ctx context.Context, scope string) (*datastore.Target, error) {
	var row rowTarget
	query := `SELECT uuid, scope, github_token, token_expired_at, resource_type, provider_url, status, status_description, created_at, updated_at FROM targets WHERE scope = ?`
	if err := m.Conn.GetContext(ctx, &row, query, scope); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	t, err := row.target()
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTargets get a all target
func (m *MySQL) ListTargets(ctx context.Context) ([]datastore.Target, error) {
	var rows []rowTarget
	query := `SELECT uuid, scope, github_token, token_expired_at, resource_type, provider_url, status, status_description, created_at, updated_at FROM targets`
	if err := m.Conn.SelectContext(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("failed to SELECT query: %w", err)
	}

	return targetsFromRows(rows)
}

// DeleteTarget delete a target
func (m *MySQL) DeleteTarget(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE targets SET status = "deleted" WHERE uuid = ?`
	if _, err := m.Conn.ExecContext(ctx, query, id.String()); err != nil {
		return fmt.Errorf("failed to execute DELETE query: %w", err)
	}

	return nil
}

// UpdateTargetStatus update status in target
func (m *MySQL) UpdateTargetStatus(ctx context.Context, targetID uuid.UUID, newStatus datastore.TargetStatus, description string) error {
	query := `UPDATE targets SET status = ?, status_description = ? WHERE uuid = ?`
	if _, err := m.Conn.ExecContext(ctx, query, newStatus, description, targetID.String()); err != nil {
		return fmt.Errorf("failed to execute UPDATE query: %w", err)
	}

	return nil
}

// UpdateToken update token in target
func (m *MySQL) UpdateToken(ctx context.Context, targetID uuid.UUID, newToken string, newExpiredAt time.Time) error {
	query := `UPDATE targets SET github_token = ?, token_expired_at = ? WHERE uuid = ?`
	if _, err := m.Conn.ExecContext(ctx, query, newToken, newExpiredAt, targetID.String()); err != nil {
		return fmt.Errorf("failed to execute UPDATE query: %w", err)
	}

	return nil
}

// UpdateTargetParam update parameter of target
func (m *MySQL) UpdateTargetParam(ctx context.Context, targetID uuid.UUID, newResourceType datastore.ResourceType, newProviderURL sql.NullString) error {
	query := `UPDATE targets SET resource_type = ?, provider_url = ? WHERE uuid = ?`
	if _, err := m.Conn.ExecContext(ctx, query, newResourceType, newProviderURL, targetID.String()); err != nil {
		return fmt.Errorf("failed to execute UPDATE query: %w", err)
	}

	return nil
}
