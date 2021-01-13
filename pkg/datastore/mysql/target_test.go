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

var testTargetID = uuid.FromStringOrNil("8a72d42c-372c-4e0d-9c6a-4304d44af137")
var testScopeRepo = "octocat/hello-world"
var testGitHubPersonalToken = "this-code-is-github-personal-token"

func TestMySQL_CreateTarget(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()
	testDB, _ := testutils.GetTestDB()

	tests := []struct {
		input datastore.Target
		want  *datastore.Target
		err   bool
	}{
		{
			input: datastore.Target{
				UUID:                testTargetID,
				Scope:               testScopeRepo,
				GitHubPersonalToken: testGitHubPersonalToken,
				GHEDomain: sql.NullString{
					Valid: false,
				},
				ResourceType: "nano",
			},
			want: &datastore.Target{
				UUID:                testTargetID,
				Scope:               testScopeRepo,
				GitHubPersonalToken: testGitHubPersonalToken,
				GHEDomain: sql.NullString{
					Valid: false,
				},
				Status:       datastore.TargetStatusInitialize,
				ResourceType: "nano",
			},
			err: false,
		},
	}

	for _, test := range tests {
		err := testDatastore.CreateTarget(context.Background(), test.input)
		if !test.err && err != nil {
			t.Fatalf("failed to create target: %+v", err)
		}
		got, err := getTargetFromSQL(testDB, test.input.UUID)
		if err != nil {
			t.Fatalf("failed to get target from SQL: %+v", err)
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

func TestMySQL_GetTarget(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:                testTargetID,
		Scope:               testScopeRepo,
		GitHubPersonalToken: testGitHubPersonalToken,
		ResourceType:        "nano",
	})
	if err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	tests := []struct {
		input uuid.UUID
		want  *datastore.Target
		err   bool
	}{
		{
			input: testTargetID,
			want: &datastore.Target{
				UUID:                testTargetID,
				Scope:               testScopeRepo,
				GitHubPersonalToken: testGitHubPersonalToken,
				Status:              datastore.TargetStatusInitialize,
				ResourceType:        "nano",
			},
			err: false,
		},
	}

	for _, test := range tests {
		got, err := testDatastore.GetTarget(context.Background(), test.input)
		if err != nil {
			t.Fatalf("failed to get target: %+v", err)
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

func TestMySQL_GetTargetByScope(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:                testTargetID,
		Scope:               testScopeRepo,
		GitHubPersonalToken: testGitHubPersonalToken,
		ResourceType:        "nano",
	})
	if err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	tests := []struct {
		input string
		want  *datastore.Target
		err   bool
	}{
		{
			input: testScopeRepo,
			want: &datastore.Target{
				UUID:                testTargetID,
				Scope:               testScopeRepo,
				GitHubPersonalToken: testGitHubPersonalToken,
				Status:              datastore.TargetStatusInitialize,
				ResourceType:        "nano",
			},
			err: false,
		},
	}

	for _, test := range tests {
		got, err := testDatastore.GetTargetByScope(context.Background(), "", test.input)
		if err != nil {
			t.Fatalf("failed to get target: %+v", err)
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

func TestMySQL_ListTargets(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:                testTargetID,
		Scope:               testScopeRepo,
		GitHubPersonalToken: testGitHubPersonalToken,
		ResourceType:        "nano",
	}); err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	tests := []struct {
		input interface{}
		want  []datastore.Target
		err   bool
	}{
		{
			input: nil,
			want: []datastore.Target{
				{
					UUID:                testTargetID,
					Scope:               testScopeRepo,
					GitHubPersonalToken: testGitHubPersonalToken,
					Status:              datastore.TargetStatusInitialize,
					ResourceType:        "nano",
				},
			},
			err: false,
		},
	}

	for _, test := range tests {
		got, err := testDatastore.ListTargets(context.Background())
		if !test.err && err != nil {
			t.Fatalf("failed to list targets: %+v", err)
		}
		if got != nil {
			for i := range got {
				got[i].CreatedAt = time.Time{}
				got[i].UpdatedAt = time.Time{}
			}
		}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestMySQL_DeleteTarget(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()
	testDB, _ := testutils.GetTestDB()

	if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:                testTargetID,
		Scope:               testScopeRepo,
		GitHubPersonalToken: testGitHubPersonalToken,
		ResourceType:        "nano",
	}); err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	tests := []struct {
		input uuid.UUID
		want  *datastore.Target
		err   bool
	}{
		{
			input: testTargetID,
			want:  nil,
			err:   false,
		},
	}

	for _, test := range tests {
		err := testDatastore.DeleteTarget(context.Background(), test.input)
		if !test.err && err != nil {
			t.Fatalf("failed to delete target: %+v", err)
		}
		got, err := getTargetFromSQL(testDB, test.input)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("failed to get target from SQL: %+v", err)
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

func TestMySQL_UpdateStatus(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()
	testDB, _ := testutils.GetTestDB()

	type Input struct {
		status      datastore.Status
		description string
	}

	tests := []struct {
		input Input
		want  *datastore.Target
		err   bool
	}{
		{
			input: Input{
				status:      datastore.TargetStatusActive,
				description: "",
			},
			want: &datastore.Target{
				UUID:                testTargetID,
				Scope:               testScopeRepo,
				GitHubPersonalToken: testGitHubPersonalToken,
				ResourceType:        "nano",
				Status:              datastore.TargetStatusActive,
				StatusDescription: sql.NullString{
					String: "",
					Valid:  true,
				},
			},
			err: false,
		},
		{
			input: Input{
				status:      datastore.TargetStatusRunning,
				description: "job-id",
			},
			want: &datastore.Target{
				UUID:                testTargetID,
				Scope:               testScopeRepo,
				GitHubPersonalToken: testGitHubPersonalToken,
				ResourceType:        "nano",
				Status:              datastore.TargetStatusRunning,
				StatusDescription: sql.NullString{
					String: "job-id",
					Valid:  true,
				},
			},
			err: false,
		},
	}

	for _, test := range tests {
		if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
			UUID:                testTargetID,
			Scope:               testScopeRepo,
			GitHubPersonalToken: testGitHubPersonalToken,
			ResourceType:        "nano",
		}); err != nil {
			t.Fatalf("failed to create target: %+v", err)
		}

		err := testDatastore.UpdateStatus(context.Background(), testTargetID, test.input.status, test.input.description)
		if !test.err && err != nil {
			t.Fatalf("failed to update status: %+v", err)
		}
		got, err := getTargetFromSQL(testDB, testTargetID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("failed to get target from SQL: %+v", err)
		}
		if got != nil {
			got.CreatedAt = time.Time{}
			got.UpdatedAt = time.Time{}
		}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}

		if err := testDatastore.DeleteTarget(context.Background(), testTargetID); err != nil {
			t.Fatalf("failed to delete target: %+v", err)
		}
	}
}

func getTargetFromSQL(testDB *sqlx.DB, uuid uuid.UUID) (*datastore.Target, error) {
	var t datastore.Target
	query := `SELECT uuid, scope, ghe_domain, github_personal_token, resource_type, runner_user, status, status_description, created_at, updated_at FROM targets WHERE uuid = ?`
	stmt, err := testDB.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare: %w", err)
	}
	err = stmt.Get(&t, uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to get target: %w", err)
	}
	return &t, nil
}
