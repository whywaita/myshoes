package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/pkg/datastore"
)

// CreateTarget create a target
func (m *MySQL) CreateTarget(ctx context.Context, target datastore.Target) error {
	expiredAtRFC3339 := target.TokenExpiredAt.Format("2006-01-02 15:04:05")

	query := `INSERT INTO targets(uuid, scope, ghe_domain, github_token, token_expired_at, resource_type, provider_url) VALUES (?, ?, ?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(
		ctx,
		query,
		target.UUID,
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
	var t datastore.Target
	query := `SELECT uuid, scope, github_token, token_expired_at, resource_type, provider_url, status, status_description, created_at, updated_at FROM targets WHERE uuid = ?`
	if err := m.Conn.GetContext(ctx, &t, query, id.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &t, nil
}

// GetTargetByScope get a target from scope
func (m *MySQL) GetTargetByScope(ctx context.Context, scope string) (*datastore.Target, error) {
	var t datastore.Target
	query := fmt.Sprintf(`SELECT uuid, scope, github_token, token_expired_at, resource_type, provider_url, status, status_description, created_at, updated_at FROM targets WHERE scope = "%s"`, scope)
	if err := m.Conn.GetContext(ctx, &t, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &t, nil
}

// ListTargets get a all target
func (m *MySQL) ListTargets(ctx context.Context) ([]datastore.Target, error) {
	var ts []datastore.Target
	query := `SELECT uuid, scope, github_token, token_expired_at, resource_type, provider_url, status, status_description, created_at, updated_at FROM targets`
	if err := m.Conn.SelectContext(ctx, &ts, query); err != nil {
		return nil, fmt.Errorf("failed to SELECT query: %w", err)
	}

	return ts, nil
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
