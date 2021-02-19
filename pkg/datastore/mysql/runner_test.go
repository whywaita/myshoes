package mysql_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jmoiron/sqlx"

	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/internal/testutils"
	"github.com/whywaita/myshoes/pkg/datastore"
)

var testRunnerID = uuid.FromStringOrNil("7943e412-c0ae-4068-ab24-3e71a13fbe53")
var testRunner = datastore.Runner{
	UUID:      testRunnerID,
	ShoesType: "shoes-test",
	IPAddress: "",
	TargetID:  testTargetID,
	CloudID:   "mycloud-uuid",
	Deleted:   false,
	Status:    datastore.RunnerStatusCreated,
}

func TestMySQL_CreateRunner(t *testing.T) {
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
		input datastore.Runner
		want  *datastore.Runner
		err   bool
	}{
		{
			input: testRunner,
			want:  &testRunner,
			err:   false,
		},
	}

	for _, test := range tests {
		err := testDatastore.CreateRunner(context.Background(), test.input)
		if !test.err && err != nil {
			t.Fatalf("failed to create runner: %+v", err)
		}
		got, err := getRunnerFromSQL(testDB, test.input.UUID)
		if err != nil {
			t.Fatalf("failed to get runner from SQL: %+v", err)
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

func TestMySQL_ListRunners(t *testing.T) {
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
		input []datastore.Runner
		want  []datastore.Runner
		err   bool
	}{
		{
			input: []datastore.Runner{
				testRunner,
			},
			want: []datastore.Runner{
				testRunner,
			},
			err: false,
		},
	}

	for _, test := range tests {
		for _, input := range test.input {
			err := testDatastore.CreateRunner(context.Background(), input)
			if !test.err && err != nil {
				t.Fatalf("failed to create runner: %+v", err)
			}
		}

		got, err := testDatastore.ListRunners(context.Background())
		if err != nil {
			t.Fatalf("failed to get runners: %+v", err)
		}
		if len(test.want) != len(got) {
			t.Fatalf("incorrect length runners, want: %d but got: %d", len(test.want), len(got))
		}
		for i := range got {
			got[i].CreatedAt = time.Time{}
			got[i].UpdatedAt = time.Time{}
		}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestMySQL_GetRunner(t *testing.T) {
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

	if err := testDatastore.CreateRunner(context.Background(), testRunner); err != nil {
		t.Fatalf("failed to create runner: %+v", err)
	}

	tests := []struct {
		input uuid.UUID
		want  *datastore.Runner
		err   bool
	}{
		{
			input: testRunnerID,
			want:  &testRunner,
			err:   false,
		},
	}

	for _, test := range tests {
		got, err := testDatastore.GetRunner(context.Background(), test.input)
		if err != nil {
			t.Fatalf("failed to get runner: %+v", err)
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

func TestMySQL_DeleteRunner(t *testing.T) {
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

	if err := testDatastore.CreateRunner(context.Background(), testRunner); err != nil {
		t.Fatalf("failed to create runner: %+v", err)
	}

	deleted := testRunner
	deleted.Deleted = true
	deleted.Status = datastore.RunnerStatusCompleted

	tests := []struct {
		input uuid.UUID
		want  *datastore.Runner
		err   bool
	}{
		{
			input: testRunnerID,
			want:  &deleted,
			err:   false,
		},
	}

	for _, test := range tests {
		err := testDatastore.DeleteRunner(context.Background(), test.input, time.Now().UTC(), datastore.RunnerStatusCompleted)
		if !test.err && err != nil {
			t.Fatalf("failed to create target: %+v", err)
		}
		got, err := getRunnerFromSQL(testDB, test.input)
		if err != nil {
			t.Fatalf("failed to get target from SQL: %+v", err)
		}
		if got != nil {
			got.CreatedAt = time.Time{}
			got.UpdatedAt = time.Time{}
			got.DeletedAt = sql.NullTime{}
		}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func getRunnerFromSQL(testDB *sqlx.DB, id uuid.UUID) (*datastore.Runner, error) {
	var r datastore.Runner
	query := `SELECT uuid, shoes_type, ip_address, target_id, cloud_id, deleted, status, created_at, updated_at, deleted_at FROM runners WHERE uuid = ?`
	stmt, err := testDB.Preparex(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare: %w", err)
	}
	err = stmt.Get(&r, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get runner: %w", err)
	}
	return &r, nil
}
