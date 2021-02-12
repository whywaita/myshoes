package mysql_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jmoiron/sqlx"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/internal/testutils"
	"github.com/whywaita/myshoes/pkg/datastore"
)

var testJobID = uuid.FromStringOrNil("1b4e5b7a-e3c1-4829-9cfd-eac4183f2c95")

func TestMySQL_EnqueueJob(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()
	testDB, _ := testutils.GetTestDB()

	if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:                testTargetID,
		Scope:               testScopeRepo,
		GitHubPersonalToken: testGitHubPersonalToken,
		ResourceType:        datastore.ResourceTypeNano,
	}); err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	tests := []struct {
		input datastore.Job
		want  *datastore.Job
		err   bool
	}{
		{
			input: datastore.Job{
				UUID:           testJobID,
				Repository:     testScopeRepo,
				CheckEventJSON: `{"example": "json"}`,
				TargetID:       testTargetID,
			},
			want: &datastore.Job{
				UUID:           testJobID,
				Repository:     testScopeRepo,
				CheckEventJSON: `{"example": "json"}`,
				TargetID:       testTargetID,
			},
			err: false,
		},
	}

	for _, test := range tests {
		err := testDatastore.EnqueueJob(context.Background(), test.input)
		if !test.err && err != nil {
			t.Fatalf("failed to enqueue job: %+v", err)
		}
		got, err := getJobFromSQL(testDB, test.input.UUID)
		if err != nil {
			t.Fatalf("failed to get job from SQL: %+v", err)
		}
		if got != nil {
			got.CreatedAt = time.Time{}
			got.UpdatedAt = time.Time{}
		}
		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestMySQL_ListJobs(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:                testTargetID,
		Scope:               testScopeRepo,
		GitHubPersonalToken: testGitHubPersonalToken,
		ResourceType:        datastore.ResourceTypeNano,
	}); err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	tests := []struct {
		input []datastore.Job
		want  []datastore.Job
		err   bool
	}{
		{
			input: []datastore.Job{
				{
					UUID:           testJobID,
					Repository:     testScopeRepo,
					CheckEventJSON: `{"example": "json"}`,
					TargetID:       testTargetID,
				},
			},
			want: []datastore.Job{
				{
					UUID:           testJobID,
					Repository:     testScopeRepo,
					CheckEventJSON: `{"example": "json"}`,
					TargetID:       testTargetID,
				},
			},
			err: false,
		},
	}

	for _, test := range tests {
		for _, input := range test.input {
			err := testDatastore.EnqueueJob(context.Background(), input)
			if !test.err && err != nil {
				t.Fatalf("failed to enqueue job: %+v", err)
			}
		}

		got, err := testDatastore.ListJobs(context.Background())
		if err != nil {
			t.Fatalf("failed to get jobs: %+v", err)
		}
		if len(test.want) != len(got) {
			t.Fatalf("incorrect length jobs, want: %d but got: %d", len(test.want), len(got))
		}
		for _, g := range got {
			g.CreatedAt = time.Time{}
			g.UpdatedAt = time.Time{}
		}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestMySQL_DeleteJob(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()
	testDB, _ := testutils.GetTestDB()

	if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:                testTargetID,
		Scope:               testScopeRepo,
		GitHubPersonalToken: testGitHubPersonalToken,
		ResourceType:        datastore.ResourceTypeNano,
	}); err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	if err := testDatastore.EnqueueJob(context.Background(), datastore.Job{
		UUID:           testJobID,
		Repository:     testScopeRepo,
		CheckEventJSON: `{"example": "json"}`,
		TargetID:       testTargetID,
	}); err != nil {
		t.Fatalf("failed to enqueue job: %+v", err)
	}

	tests := []struct {
		input uuid.UUID
		want  *datastore.Job
		err   bool
	}{
		{
			input: testJobID,
			want:  nil,
			err:   false,
		},
	}

	for _, test := range tests {
		err := testDatastore.DeleteJob(context.Background(), test.input)
		if !test.err && err != nil {
			t.Fatalf("failed to delete job: %+v", err)
		}

		got, err := getJobFromSQL(testDB, test.input)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("failed to get job from SQL: %+v", err)
		}
		if got != nil {
			got.CreatedAt = time.Time{}
			got.UpdatedAt = time.Time{}
		}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func getJobFromSQL(testDB *sqlx.DB, id uuid.UUID) (*datastore.Job, error) {
	var j datastore.Job
	query := `SELECT uuid, ghe_domain, repository, check_event, target_id FROM jobs WHERE uuid = ?`
	stmt, err := testDB.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare: %w", err)
	}
	err = stmt.Get(&j, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return &j, nil
}

func listJobFromSQL(testDB *sqlx.DB) ([]datastore.Job, error) {
	var j []datastore.Job
	query := `SELECT uuid, ghe_domain, repository, check_event, target_id FROM jobs`
	stmt, err := testDB.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare: %w", err)
	}
	err = stmt.Select(&j)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return j, nil
}
