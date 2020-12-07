package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	query := `INSERT INTO targets(uuid, scope, ghe_domain, github_personal_token, resource_type) VALUES (?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(ctx, query, target.UUID, target.Scope, target.GHEDomain, target.GitHubPersonalToken, target.ResourceType); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

func (m *MySQL) GetTarget(ctx context.Context, id uuid.UUID) (*datastore.Target, error) {
	var t datastore.Target
	query := fmt.Sprintf(`SELECT uuid, scope, ghe_domain, github_personal_token, resource_type, created_at, updated_at FROM targets WHERE uuid = "%s"`, id.String())
	if err := m.Conn.GetContext(ctx, &t, query); err != nil {
		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &t, nil
}

func (m *MySQL) GetTargetByScope(ctx context.Context, gheDomain, scope string) (*datastore.Target, error) {
	var t datastore.Target
	query := fmt.Sprintf(`SELECT uuid, scope, ghe_domain, github_personal_token, resource_type, created_at, updated_at FROM targets WHERE scope = "%s"`, scope)
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
	query := `DELETE FROM targets WHERE uuid = "?"`
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

func (m *MySQL) GetJob(ctx context.Context) ([]datastore.Job, error) {
	var jobs []datastore.Job
	query := `SELECT uuid, ghe_domain, repository, check_event, target_id FROM jobs`
	if err := m.Conn.SelectContext(ctx, &jobs, query); err != nil {
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
