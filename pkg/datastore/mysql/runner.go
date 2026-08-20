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

// rowRunner mirrors datastore.Runner but stores the UUID columns (runner_id,
// target_id) as strings so that database/sql can scan the VARCHAR(36) columns.
// The standard library uuid.UUID is a [16]byte with no sql.Scanner.
type rowRunner struct {
	UUID           string                 `db:"runner_id"`
	ShoesType      string                 `db:"shoes_type"`
	IPAddress      string                 `db:"ip_address"`
	TargetID       string                 `db:"target_id"`
	CloudID        string                 `db:"cloud_id"`
	ResourceType   datastore.ResourceType `db:"resource_type"`
	RunnerUser     sql.NullString         `db:"runner_user"`
	ProviderURL    sql.NullString         `db:"provider_url"`
	RepositoryURL  string                 `db:"repository_url"`
	RequestWebhook string                 `db:"request_webhook"`
	CreatedAt      time.Time              `db:"created_at"`
	UpdatedAt      time.Time              `db:"updated_at"`
}

func (r rowRunner) runner() (datastore.Runner, error) {
	u, err := uuid.Parse(r.UUID)
	if err != nil {
		return datastore.Runner{}, fmt.Errorf("failed to parse runner uuid %q: %w", r.UUID, err)
	}

	tid, err := uuid.Parse(r.TargetID)
	if err != nil {
		return datastore.Runner{}, fmt.Errorf("failed to parse target id %q: %w", r.TargetID, err)
	}

	return datastore.Runner{
		UUID:           u,
		ShoesType:      r.ShoesType,
		IPAddress:      r.IPAddress,
		TargetID:       tid,
		CloudID:        r.CloudID,
		ResourceType:   r.ResourceType,
		RunnerUser:     r.RunnerUser,
		ProviderURL:    r.ProviderURL,
		RepositoryURL:  r.RepositoryURL,
		RequestWebhook: r.RequestWebhook,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}, nil
}

func runnersFromRows(rows []rowRunner) ([]datastore.Runner, error) {
	rs := make([]datastore.Runner, 0, len(rows))
	for _, r := range rows {
		runner, err := r.runner()
		if err != nil {
			return nil, err
		}
		rs = append(rs, runner)
	}
	return rs, nil
}

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
	var rows []rowRunner
	query := `SELECT runner.runner_id, detail.shoes_type, detail.ip_address, detail.target_id, detail.cloud_id, detail.created_at, detail.updated_at, detail.resource_type, detail.repository_url, detail.request_webhook, detail.runner_user, detail.provider_url
 FROM runners_running AS runner JOIN runner_detail AS detail ON runner.runner_id = detail.runner_id`
	err := m.Conn.SelectContext(ctx, &rows, query)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return runnersFromRows(rows)
}

// ListRunnersByTargetID get a not deleted runners that has target_id
func (m *MySQL) ListRunnersByTargetID(ctx context.Context, targetID uuid.UUID) ([]datastore.Runner, error) {
	var rows []rowRunner
	query := `SELECT runner.runner_id, detail.shoes_type, detail.ip_address, detail.target_id, detail.cloud_id, detail.created_at, detail.updated_at, detail.resource_type, detail.repository_url, detail.request_webhook, detail.runner_user, detail.provider_url
 FROM runners_running AS runner JOIN runner_detail AS detail ON runner.runner_id = detail.runner_id WHERE detail.target_id = ?`
	err := m.Conn.SelectContext(ctx, &rows, query, targetID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return runnersFromRows(rows)
}

// ListRunnersLogBySince ListRunnerLog get a runners since time
func (m *MySQL) ListRunnersLogBySince(ctx context.Context, since time.Time) ([]datastore.Runner, error) {
	var rows []rowRunner

	query := `SELECT runner_id, shoes_type, ip_address, target_id, cloud_id, created_at, updated_at, resource_type, repository_url, request_webhook, runner_user, provider_url FROM runner_detail WHERE created_at > ?`
	err := m.Conn.SelectContext(ctx, &rows, query, since)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return runnersFromRows(rows)
}

// GetRunner get a runner
func (m *MySQL) GetRunner(ctx context.Context, id uuid.UUID) (*datastore.Runner, error) {
	var row rowRunner

	query := `SELECT runner_id, shoes_type, ip_address, target_id, cloud_id, created_at, updated_at, resource_type, repository_url, request_webhook, runner_user, provider_url FROM runner_detail WHERE runner_id = ?`
	if err := m.Conn.GetContext(ctx, &row, query, id.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	r, err := row.runner()
	if err != nil {
		return nil, err
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
