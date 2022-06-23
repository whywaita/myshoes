package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/pkg/datastore"
)

// EnqueueJob add a job
func (m *MySQL) EnqueueJob(ctx context.Context, job datastore.Job) error {
	query := `INSERT INTO jobs(uuid, ghe_domain, repository, check_event, target_id, owner) VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(ctx, query, job.UUID, job.GHEDomain, job.Repository, job.CheckEventJSON, job.TargetID.String(), job.Owner); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

// ListJobs get all jobs
func (m *MySQL) ListJobs(ctx context.Context) ([]datastore.Job, error) {
	var jobs []datastore.Job
	query := `SELECT uuid, ghe_domain, repository, check_event, target_id, owner, created_at, updated_at FROM jobs`
	if err := m.Conn.SelectContext(ctx, &jobs, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return jobs, nil
}

// DequeueJobs dequeue jobs
func (m *MySQL) DequeueJobs(ctx context.Context) ([]datastore.Job, error) {
	connectionID, err := m.getConnectionID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection id: %w", err)
	}

	tx := m.Conn.MustBegin()

	var noOwnerUUID []string
	noOwnerQuery := `SELECT uuid FROM jobs WHERE owner IS NULL`
	if err := tx.SelectContext(ctx, &noOwnerUUID, noOwnerQuery); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get a list of unowner jobs: %w", err)
	}

	if len(noOwnerUUID) != 0 {
		hasOwnerQueryRaw := fmt.Sprintf(`UPDATE jobs SET owner = %d WHERE uuid IN (?)`, connectionID)
		hasOwnerQuery, params, err := sqlx.In(hasOwnerQueryRaw, noOwnerUUID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to prepare query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, hasOwnerQuery, params...); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to UPDATE owner: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to execute COMMIT(): %w", err)
	}

	ownedQuery := `SELECT uuid, ghe_domain, repository, check_event, target_id, owner, created_at, updated_at FROM jobs WHERE owner = ?`
	var ownedJobs []datastore.Job
	if err := m.Conn.SelectContext(ctx, &ownedJobs, ownedQuery, connectionID); err != nil {
		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return ownedJobs, nil
}

// DeleteJob delete a job
func (m *MySQL) DeleteJob(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`DELETE FROM jobs WHERE uuid = ?`)
	if _, err := m.Conn.ExecContext(ctx, query, id.String()); err != nil {
		return fmt.Errorf("failed to execute DELETE query: %w", err)
	}

	return nil
}

// CleanUpJobs cleanup owner if owner is already disconnected
func (m *MySQL) CleanUpJobs(ctx context.Context) error {
	query := `UPDATE jobs SET owner = NULL WHERE owner NOT IN (SELECT ID FROM information_schema.processlist)`
	if _, err := m.Conn.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to execute UPDATE query: %w", err)
	}

	return nil
}
