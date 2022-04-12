package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/pkg/datastore"
)

// EnqueueJob add a job
func (m *MySQL) EnqueueJob(ctx context.Context, job datastore.Job) error {
	query := `INSERT INTO jobs(uuid, ghe_domain, repository, check_event, target_id) VALUES (?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(ctx, query, job.UUID, job.GHEDomain, job.Repository, job.CheckEventJSON, job.TargetID.String()); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

// ListJobs get all jobs
func (m *MySQL) ListJobs(ctx context.Context) ([]datastore.Job, error) {
	var jobs []datastore.Job
	query := `SELECT uuid, ghe_domain, repository, check_event, target_id, created_at, updated_at FROM jobs`
	if err := m.Conn.SelectContext(ctx, &jobs, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return jobs, nil
}

// DeleteJob delete a job
func (m *MySQL) DeleteJob(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`DELETE FROM jobs WHERE uuid = ?`)
	if _, err := m.Conn.ExecContext(ctx, query, id.String()); err != nil {
		return fmt.Errorf("failed to execute DELETE query: %w", err)
	}

	return nil
}
