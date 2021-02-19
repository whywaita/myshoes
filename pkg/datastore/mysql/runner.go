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

// CreateRunner add a runner
func (m *MySQL) CreateRunner(ctx context.Context, runner datastore.Runner) error {
	query := `INSERT INTO runners(uuid, shoes_type, ip_address, target_id, cloud_id, deleted) VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(ctx, query, runner.UUID.String(), runner.ShoesType, runner.IPAddress, runner.TargetID.String(), runner.CloudID, false); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

// ListRunners get a not deleted runners
func (m *MySQL) ListRunners(ctx context.Context) ([]datastore.Runner, error) {
	var runners []datastore.Runner
	query := `SELECT uuid, shoes_type, ip_address, target_id, cloud_id, deleted, status, created_at, updated_at, deleted_at FROM runners WHERE deleted = false`
	err := m.Conn.SelectContext(ctx, &runners, query)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return runners, nil
}

// GetRunner get a runner
func (m *MySQL) GetRunner(ctx context.Context, id uuid.UUID) (*datastore.Runner, error) {
	var r datastore.Runner

	query := `SELECT uuid, shoes_type, ip_address, target_id, cloud_id, deleted, status, created_at, updated_at, deleted_at FROM runners WHERE uuid = ? AND deleted = false`
	if err := m.Conn.GetContext(ctx, &r, query, id.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &r, nil
}

// DeleteRunner delete a runner
func (m *MySQL) DeleteRunner(ctx context.Context, id uuid.UUID, deletedAt time.Time, reason datastore.RunnerStatus) error {
	query := `UPDATE runners SET deleted=1, deleted_at = ?, status = ? WHERE uuid = ? `
	if _, err := m.Conn.ExecContext(ctx, query, deletedAt, reason, id.String()); err != nil {
		return fmt.Errorf("failed to execute UPDATE query: %w", err)
	}

	return nil
}
