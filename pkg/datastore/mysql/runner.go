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
	tx := m.Conn.MustBegin()

	queryRunner := `INSERT INTO runners(uuid) VALUES (?)`
	if _, err := tx.ExecContext(ctx, queryRunner, runner.UUID.String()); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute INSERT query runners: %w", err)
	}

	queryDetail := `INSERT INTO runner_detail(runner_id, shoes_type, ip_address, target_id, cloud_id, resource_type, runner_user, repository_url, request_webhook, provider_url) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := tx.ExecContext(ctx, queryDetail, runner.UUID.String(), runner.ShoesType, runner.IPAddress, runner.TargetID.String(), runner.CloudID, runner.ResourceType, runner.RunnerUser, runner.RepositoryURL, runner.RequestWebhook, runner.ProviderURL); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute INSERT query runner_detail: %w", err)
	}

	queryRunning := `INSERT INTO runners_running(runner_id) VALUES (?)`
	if _, err := tx.ExecContext(ctx, queryRunning, runner.UUID.String()); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute INSERT query runners_running: %w", err)
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute COMMIT: %w", err)
	}
	return nil
}

// ListRunners get a not deleted runners
func (m *MySQL) ListRunners(ctx context.Context) ([]datastore.Runner, error) {
	var runners []datastore.Runner
	query := `SELECT runner.runner_id, detail.shoes_type, detail.ip_address, detail.target_id, detail.cloud_id, detail.created_at, detail.updated_at, detail.resource_type, detail.repository_url, detail.request_webhook, detail.runner_user, detail.provider_url
 FROM runners_running AS runner JOIN runner_detail AS detail ON runner.runner_id = detail.runner_id`
	err := m.Conn.SelectContext(ctx, &runners, query)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return runners, nil
}

// ListRunnersByTargetID get a not deleted runners that has target_id
func (m *MySQL) ListRunnersByTargetID(ctx context.Context, targetID uuid.UUID) ([]datastore.Runner, error) {
	var runners []datastore.Runner
	query := `SELECT runner.runner_id, detail.shoes_type, detail.ip_address, detail.target_id, detail.cloud_id, detail.created_at, detail.updated_at, detail.resource_type, detail.repository_url, detail.request_webhook, detail.runner_user, detail.provider_url
 FROM runners_running AS runner JOIN runner_detail AS detail ON runner.runner_id = detail.runner_id WHERE detail.target_id = ?`
	err := m.Conn.SelectContext(ctx, &runners, query, targetID)
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

	query := `SELECT runner_id, shoes_type, ip_address, target_id, cloud_id, created_at, updated_at, resource_type, repository_url, request_webhook, runner_user, provider_url FROM runner_detail WHERE runner_id = ?`
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
	tx := m.Conn.MustBegin()

	queryDelete := `DELETE FROM runners_running WHERE runner_id = ?`
	if _, err := tx.ExecContext(ctx, queryDelete, id.String()); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute DELETE query: %w", err)
	}

	queryInsert := `INSERT INTO runners_deleted(runner_id, reason) VALUES (?, ?)`
	if _, err := tx.ExecContext(ctx, queryInsert, id.String(), reason); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute COMMIT: %w", err)
	}

	return nil
}
