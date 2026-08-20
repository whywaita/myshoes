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

// rowJob mirrors datastore.Job but stores the UUID columns (uuid, target_id) as
// strings so that database/sql can scan the VARCHAR(36) columns. The standard
// library uuid.UUID is a [16]byte with no sql.Scanner.
type rowJob struct {
	UUID           string         `db:"uuid"`
	GHEDomain      sql.NullString `db:"ghe_domain"`
	Repository     string         `db:"repository"`
	CheckEventJSON string         `db:"check_event"`
	TargetID       string         `db:"target_id"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}

func (r rowJob) job() (datastore.Job, error) {
	u, err := uuid.Parse(r.UUID)
	if err != nil {
		return datastore.Job{}, fmt.Errorf("failed to parse job uuid %q: %w", r.UUID, err)
	}

	tid, err := uuid.Parse(r.TargetID)
	if err != nil {
		return datastore.Job{}, fmt.Errorf("failed to parse target id %q: %w", r.TargetID, err)
	}

	return datastore.Job{
		UUID:           u,
		GHEDomain:      r.GHEDomain,
		Repository:     r.Repository,
		CheckEventJSON: r.CheckEventJSON,
		TargetID:       tid,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}, nil
}

func jobsFromRows(rows []rowJob) ([]datastore.Job, error) {
	js := make([]datastore.Job, 0, len(rows))
	for _, r := range rows {
		j, err := r.job()
		if err != nil {
			return nil, err
		}
		js = append(js, j)
	}
	return js, nil
}

// EnqueueJob add a job
func (m *MySQL) EnqueueJob(ctx context.Context, job datastore.Job) error {
	query := `INSERT INTO jobs(uuid, ghe_domain, repository, check_event, target_id) VALUES (?, ?, ?, ?, ?)`
	if _, err := m.Conn.ExecContext(ctx, query, job.UUID.String(), job.GHEDomain, job.Repository, job.CheckEventJSON, job.TargetID.String()); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	select {
	case m.notifyEnqueueCh <- struct{}{}:
		// notified to starter
	default:
		// no capacity on channel, do not block
	}

	return nil
}

// ListJobs get all jobs
func (m *MySQL) ListJobs(ctx context.Context) ([]datastore.Job, error) {
	var rows []rowJob
	query := `SELECT uuid, ghe_domain, repository, check_event, target_id, created_at, updated_at FROM jobs`
	if err := m.Conn.SelectContext(ctx, &rows, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return jobsFromRows(rows)
}

// DeleteJob delete a job
func (m *MySQL) DeleteJob(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM jobs WHERE uuid = ?`
	if _, err := m.Conn.ExecContext(ctx, query, id.String()); err != nil {
		return fmt.Errorf("failed to execute DELETE query: %w", err)
	}

	return nil
}
