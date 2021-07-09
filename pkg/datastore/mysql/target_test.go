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
var testGitHubToken = "this-code-is-github-token"
var testRunnerVersion = "v999.99.9"
var testProviderURL = "/shoes-mock"
var testTime = time.Date(2037, 9, 3, 0, 0, 0, 0, time.UTC)

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
				UUID:           testTargetID,
				Scope:          testScopeRepo,
				GitHubToken:    testGitHubToken,
				TokenExpiredAt: testTime,
				GHEDomain: sql.NullString{
					Valid: false,
				},
				ResourceType: datastore.ResourceTypeNano,
				RunnerVersion: sql.NullString{
					String: testRunnerVersion,
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: testProviderURL,
					Valid:  true,
				},
			},
			want: &datastore.Target{
				UUID:           testTargetID,
				Scope:          testScopeRepo,
				GitHubToken:    testGitHubToken,
				TokenExpiredAt: testTime,
				GHEDomain: sql.NullString{
					Valid: false,
				},
				Status:       datastore.TargetStatusActive,
				ResourceType: datastore.ResourceTypeNano,
				RunnerVersion: sql.NullString{
					String: testRunnerVersion,
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: testProviderURL,
					Valid:  true,
				},
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
		UUID:           testTargetID,
		Scope:          testScopeRepo,
		GitHubToken:    testGitHubToken,
		TokenExpiredAt: testTime,
		ResourceType:   datastore.ResourceTypeNano,
		RunnerVersion: sql.NullString{
			String: testRunnerVersion,
			Valid:  true,
		},
		ProviderURL: sql.NullString{
			String: testProviderURL,
			Valid:  true,
		},
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
				UUID:           testTargetID,
				Scope:          testScopeRepo,
				GitHubToken:    testGitHubToken,
				TokenExpiredAt: testTime,
				Status:         datastore.TargetStatusActive,
				ResourceType:   datastore.ResourceTypeNano,
				RunnerVersion: sql.NullString{
					String: testRunnerVersion,
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: testProviderURL,
					Valid:  true,
				},
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
		UUID:           testTargetID,
		Scope:          testScopeRepo,
		GitHubToken:    testGitHubToken,
		TokenExpiredAt: testTime,
		ResourceType:   datastore.ResourceTypeNano,
		RunnerVersion: sql.NullString{
			String: testRunnerVersion,
			Valid:  true,
		},
		ProviderURL: sql.NullString{
			String: testProviderURL,
			Valid:  true,
		},
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
				UUID:           testTargetID,
				Scope:          testScopeRepo,
				GitHubToken:    testGitHubToken,
				TokenExpiredAt: testTime,
				Status:         datastore.TargetStatusActive,
				ResourceType:   datastore.ResourceTypeNano,
				RunnerVersion: sql.NullString{
					String: testRunnerVersion,
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: testProviderURL,
					Valid:  true,
				},
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
		UUID:           testTargetID,
		Scope:          testScopeRepo,
		GitHubToken:    testGitHubToken,
		TokenExpiredAt: testTime,
		ResourceType:   datastore.ResourceTypeNano,
		RunnerVersion: sql.NullString{
			String: testRunnerVersion,
			Valid:  true,
		},
		ProviderURL: sql.NullString{
			String: testProviderURL,
			Valid:  true,
		},
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
					UUID:           testTargetID,
					Scope:          testScopeRepo,
					GitHubToken:    testGitHubToken,
					TokenExpiredAt: testTime,
					Status:         datastore.TargetStatusActive,
					ResourceType:   datastore.ResourceTypeNano,
					RunnerVersion: sql.NullString{
						String: testRunnerVersion,
						Valid:  true,
					},
					ProviderURL: sql.NullString{
						String: testProviderURL,
						Valid:  true,
					},
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
		UUID:           testTargetID,
		Scope:          testScopeRepo,
		GitHubToken:    testGitHubToken,
		TokenExpiredAt: testTime,
		ResourceType:   datastore.ResourceTypeNano,
		RunnerVersion: sql.NullString{
			String: testRunnerVersion,
			Valid:  true,
		},
		ProviderURL: sql.NullString{
			String: testProviderURL,
			Valid:  true,
		},
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
			want: &datastore.Target{
				UUID:           testTargetID,
				Scope:          testScopeRepo,
				GitHubToken:    testGitHubToken,
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeNano,
				RunnerVersion: sql.NullString{
					String: testRunnerVersion,
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: testProviderURL,
					Valid:  true,
				},
				Status: datastore.TargetStatusDeleted,
			},
			err: false,
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
		status      datastore.TargetStatus
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
				Scope:          testScopeRepo,
				GitHubToken:    testGitHubToken,
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeNano,
				RunnerVersion: sql.NullString{
					String: testRunnerVersion,
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: testProviderURL,
					Valid:  true,
				},
				Status: datastore.TargetStatusActive,
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
				Scope:          testScopeRepo,
				GitHubToken:    testGitHubToken,
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeNano,
				RunnerVersion: sql.NullString{
					String: testRunnerVersion,
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: testProviderURL,
					Valid:  true,
				},
				Status: datastore.TargetStatusRunning,
				StatusDescription: sql.NullString{
					String: "job-id",
					Valid:  true,
				},
			},
			err: false,
		},
	}

	for _, test := range tests {
		tID := uuid.NewV4()
		if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
			UUID:           tID,
			Scope:          testScopeRepo,
			GitHubToken:    testGitHubToken,
			TokenExpiredAt: testTime,
			ResourceType:   datastore.ResourceTypeNano,
			RunnerVersion: sql.NullString{
				String: testRunnerVersion,
				Valid:  true,
			},
			ProviderURL: sql.NullString{
				String: testProviderURL,
				Valid:  true,
			},
		}); err != nil {
			t.Fatalf("failed to create target: %+v", err)
		}

		err := testDatastore.UpdateTargetStatus(context.Background(), tID, test.input.status, test.input.description)
		if !test.err && err != nil {
			t.Fatalf("failed to update status: %+v", err)
		}
		got, err := getTargetFromSQL(testDB, tID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("failed to get target from SQL: %+v", err)
		}
		if got != nil {
			got.UUID = uuid.UUID{}
			got.CreatedAt = time.Time{}
			got.UpdatedAt = time.Time{}
		}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}

		if err := testDatastore.DeleteTarget(context.Background(), tID); err != nil {
			t.Fatalf("failed to delete target: %+v", err)
		}
	}
}

func TestMySQL_UpdateToken(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()
	testDB, _ := testutils.GetTestDB()

	type Input struct {
		token   string
		expired time.Time
	}

	tests := []struct {
		input Input
		want  *datastore.Target
		err   bool
	}{
		{
			input: Input{
				token:   "new-token",
				expired: testTime.Add(1 * time.Hour),
			},
			want: &datastore.Target{
				Scope:          testScopeRepo,
				GitHubToken:    "new-token",
				TokenExpiredAt: testTime.Add(1 * time.Hour),
				ResourceType:   datastore.ResourceTypeNano,
				RunnerVersion: sql.NullString{
					String: testRunnerVersion,
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: testProviderURL,
					Valid:  true,
				},
				Status: datastore.TargetStatusActive,
				StatusDescription: sql.NullString{
					String: "",
					Valid:  false,
				},
			},
			err: false,
		},
	}

	for _, test := range tests {
		tID := uuid.NewV4()
		if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
			UUID:           tID,
			Scope:          testScopeRepo,
			GitHubToken:    testGitHubToken,
			TokenExpiredAt: testTime,
			ResourceType:   datastore.ResourceTypeNano,
			RunnerVersion: sql.NullString{
				String: testRunnerVersion,
				Valid:  true,
			},
			ProviderURL: sql.NullString{
				String: testProviderURL,
				Valid:  true,
			},
		}); err != nil {
			t.Fatalf("failed to create target: %+v", err)
		}

		err := testDatastore.UpdateToken(context.Background(), tID, test.input.token, test.input.expired)
		if !test.err && err != nil {
			t.Fatalf("failed to update status: %+v", err)
		}
		got, err := getTargetFromSQL(testDB, tID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("failed to get target from SQL: %+v", err)
		}
		if got != nil {
			got.UUID = uuid.UUID{}
			got.CreatedAt = time.Time{}
			got.UpdatedAt = time.Time{}
		}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}

		if err := testDatastore.DeleteTarget(context.Background(), tID); err != nil {
			t.Fatalf("failed to delete target: %+v", err)
		}
	}
}

func getTargetFromSQL(testDB *sqlx.DB, uuid uuid.UUID) (*datastore.Target, error) {
	var t datastore.Target
	query := `SELECT uuid, scope, ghe_domain, github_token, token_expired_at, resource_type, runner_user, runner_version, provider_url, status, status_description, created_at, updated_at FROM targets WHERE uuid = ?`
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
