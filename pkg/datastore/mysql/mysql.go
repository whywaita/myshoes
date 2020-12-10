package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/pkg/datastore"

	// mysql driver
	_ "github.com/go-sql-driver/mysql"
)

type MySQL struct {
	Conn *sqlx.DB
}

// New create mysql connection
func New(dsn string) (*MySQL, error) {
	conn, err := sqlx.Open("mysql", dsn+"?parseTime=true")
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql connection: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to conn.Ping: %w", err)
	}

	return &MySQL{
		Conn: conn,
	}, nil
}

func (m *MySQL) CreateTarget(ctx context.Context, target datastore.Target) error {
	query := `INSERT INTO targets(uuid, scope, ghe_domain, github_personal_token, resource_type, runner_user) VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(ctx, query, target.UUID, target.Scope, target.GHEDomain, target.GitHubPersonalToken, target.ResourceType, target.RunnerUser); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

func (m *MySQL) GetTarget(ctx context.Context, id uuid.UUID) (*datastore.Target, error) {
	var t datastore.Target
	query := fmt.Sprintf(`SELECT uuid, scope, ghe_domain, github_personal_token, resource_type, runner_user, created_at, updated_at FROM targets WHERE uuid = ?`)
	if err := m.Conn.GetContext(ctx, &t, query, id.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &t, nil
}

func (m *MySQL) GetTargetByScope(ctx context.Context, gheDomain, scope string) (*datastore.Target, error) {
	var t datastore.Target
	query := fmt.Sprintf(`SELECT uuid, scope, ghe_domain, github_personal_token, resource_type, runner_user, created_at, updated_at FROM targets WHERE scope = "%s"`, scope)
	if gheDomain != "" {
		query = fmt.Sprintf(`%s AND ghe_domain = "%s"`, query, gheDomain)
	}
	if err := m.Conn.GetContext(ctx, &t, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &t, nil
}

func (m *MySQL) DeleteTarget(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM targets WHERE uuid = ?`
	if _, err := m.Conn.ExecContext(ctx, query, id.String()); err != nil {
		return fmt.Errorf("failed to execute DELETE query: %w", err)
	}

	return nil
}

func (m *MySQL) EnqueueJob(ctx context.Context, job datastore.Job) error {
	query := `INSERT INTO jobs(uuid, ghe_domain, repository, check_event, target_id) VALUES (?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(ctx, query, job.UUID, job.GHEDomain, job.Repository, job.CheckEventJSON, job.TargetID.String()); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

func (m *MySQL) ListJobs(ctx context.Context) ([]datastore.Job, error) {
	var jobs []datastore.Job
	query := `SELECT uuid, ghe_domain, repository, check_event, target_id FROM jobs`
	if err := m.Conn.SelectContext(ctx, &jobs, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return jobs, nil
}

func (m *MySQL) DeleteJob(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`DELETE FROM jobs WHERE uuid = ?`)
	if _, err := m.Conn.ExecContext(ctx, query, id.String()); err != nil {
		return fmt.Errorf("failed to execute DELETE query: %w", err)
	}

	return nil
}

func (m *MySQL) CreateRunner(ctx context.Context, runner datastore.Runner) error {
	query := `INSERT INTO runners(uuid, shoes_type, ip_address, target_id, cloud_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(ctx, query, runner.UUID.String(), runner.ShoesType, runner.IPAddress, runner.TargetID.String(), runner.CloudID, runner.CreatedAt, runner.UpdatedAt); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

func (m *MySQL) ListRunners(ctx context.Context) ([]datastore.Runner, error) {
	var runners []datastore.Runner
	query := `SELECT uuid, shoes_type, ip_address, target_id, cloud_id, deleted, created_at, updated_at, deleted_at FROM runners WHERE deleted = 0`
	err := m.Conn.SelectContext(ctx, &runners, query)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return runners, nil
}

func (m *MySQL) GetRunner(ctx context.Context, id uuid.UUID) (*datastore.Runner, error) {
	var r datastore.Runner

	query := `SELECT uuid, shoes_type, ip_address, target_id, cloud_id, deleted, created_at, updated_at, deleted_at FROM runners WHERE uuid = ? AND deleted = 0`
	if err := m.Conn.GetContext(ctx, &r, query, id.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &r, nil
}

func (m *MySQL) DeleteRunner(ctx context.Context, id uuid.UUID, deletedAt time.Time) error {
	query := `UPDATE runners SET deleted=1, deleted_at = ? WHERE uuid = ? `
	if _, err := m.Conn.ExecContext(ctx, query, deletedAt, id.String()); err != nil {
		return fmt.Errorf("failed to execute UPDATE query: %w", err)
	}

	return nil
}
