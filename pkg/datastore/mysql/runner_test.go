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

var testRunnerID = uuid.FromStringOrNil("7943e412-c0ae-4068-ab24-3e71a13fbe53")

func TestMySQL_CreateRunner(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()
	testDB, _ := testutils.GetTestDB()

	tests := []struct {
		input datastore.Runner
		want  *datastore.Runner
		err   bool
	}{
		{
			input: datastore.Runner{
				UUID:           testRunnerID,
				ShoesType:      "shoes-test",
				TargetID:       testTargetID,
				CloudID:        "mycloud-uuid",
				ResourceType:   datastore.ResourceTypeNano,
				RepositoryURL:  "https://github.com/octocat/Hello-World",
				RequestWebhook: "{}",
			},
			want: &datastore.Runner{
				UUID:           testRunnerID,
				ShoesType:      "shoes-test",
				TargetID:       testTargetID,
				CloudID:        "mycloud-uuid",
				ResourceType:   datastore.ResourceTypeNano,
				RepositoryURL:  "https://github.com/octocat/Hello-World",
				RequestWebhook: "{}",
			},
			err: false,
		},
		{
			input: datastore.Runner{
				UUID:         testRunnerID,
				ShoesType:    "shoes-test",
				TargetID:     testTargetID,
				CloudID:      "mycloud-uuid",
				ResourceType: datastore.ResourceTypeNano,
				RunnerUser: sql.NullString{
					String: "runner",
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: "./shoes-test",
					Valid:  true,
				},
				RepositoryURL:  "https://github.com/octocat/Hello-World",
				RequestWebhook: "{}",
			},
			want: &datastore.Runner{
				UUID:         testRunnerID,
				ShoesType:    "shoes-test",
				TargetID:     testTargetID,
				CloudID:      "mycloud-uuid",
				ResourceType: datastore.ResourceTypeNano,
				RunnerUser: sql.NullString{
					String: "runner",
					Valid:  true,
				},
				ProviderURL: sql.NullString{
					String: "./shoes-test",
					Valid:  true,
				},
				RepositoryURL:  "https://github.com/octocat/Hello-World",
				RequestWebhook: "{}",
			},
			err: false,
		},
	}

	for _, test := range tests {
		if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
			UUID:           testTargetID,
			Scope:          testScopeRepo,
			GitHubToken:    testGitHubToken,
			TokenExpiredAt: testTime,
			ResourceType:   datastore.ResourceTypeNano,
		}); err != nil {
			t.Fatalf("failed to create target: %+v", err)
		}

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

		teardown()
	}
}

func TestMySQL_ListRunners(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:           testTargetID,
		Scope:          testScopeRepo,
		GitHubToken:    testGitHubToken,
		TokenExpiredAt: testTime,
		ResourceType:   datastore.ResourceTypeNano,
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
				{
					UUID:           testRunnerID,
					ShoesType:      "shoes-test",
					TargetID:       testTargetID,
					CloudID:        "mycloud-uuid",
					ResourceType:   datastore.ResourceTypeNano,
					RepositoryURL:  "https://github.com/octocat/Hello-World",
					RequestWebhook: "{}",
				},
			},
			want: []datastore.Runner{
				{
					UUID:           testRunnerID,
					ShoesType:      "shoes-test",
					TargetID:       testTargetID,
					CloudID:        "mycloud-uuid",
					ResourceType:   datastore.ResourceTypeNano,
					RepositoryURL:  "https://github.com/octocat/Hello-World",
					RequestWebhook: "{}",
				},
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

func TestMySQL_ListRunnersNotReturnDeleted(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:           testTargetID,
		Scope:          testScopeRepo,
		GitHubToken:    testGitHubToken,
		TokenExpiredAt: testTime,
		ResourceType:   datastore.ResourceTypeNano,
	}); err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	u := "00000000-0000-0000-0000-00000000000%d"

	for i := 0; i < 3; i++ {
		input := datastore.Runner{
			UUID:           testRunnerID,
			ShoesType:      "shoes-test",
			TargetID:       testTargetID,
			CloudID:        "mycloud-uuid",
			ResourceType:   datastore.ResourceTypeNano,
			RepositoryURL:  "https://github.com/octocat/Hello-World",
			RequestWebhook: "{}",
		}
		input.UUID = uuid.FromStringOrNil(fmt.Sprintf(u, i))
		err := testDatastore.CreateRunner(context.Background(), input)
		if err != nil {
			t.Fatalf("failed to create runner: %+v", err)
		}
	}

	err := testDatastore.DeleteRunner(context.Background(), uuid.FromStringOrNil(fmt.Sprintf(u, 0)), time.Now(), "deleted")
	if err != nil {
		t.Fatalf("failed to delete runner: %+v", err)
	}

	got, err := testDatastore.ListRunners(context.Background())
	if err != nil {
		t.Fatalf("failed to get runners: %+v", err)
	}
	for i := range got {
		got[i].CreatedAt = time.Time{}
		got[i].UpdatedAt = time.Time{}
	}

	var want []datastore.Runner
	for i := 1; i < 3; i++ {
		r := datastore.Runner{
			UUID:           testRunnerID,
			ShoesType:      "shoes-test",
			TargetID:       testTargetID,
			CloudID:        "mycloud-uuid",
			ResourceType:   datastore.ResourceTypeNano,
			RepositoryURL:  "https://github.com/octocat/Hello-World",
			RequestWebhook: "{}",
		}
		r.UUID = uuid.FromStringOrNil(fmt.Sprintf(u, i))
		want = append(want, r)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestMySQL_ListRunnersLogBySince(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:           testTargetID,
		Scope:          testScopeRepo,
		GitHubToken:    testGitHubToken,
		TokenExpiredAt: testTime,
		ResourceType:   datastore.ResourceTypeNano,
	}); err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	u := "00000000-0000-0000-0000-00000000000%d"

	for i := 1; i < 3; i++ {
		input := datastore.Runner{
			UUID:           testRunnerID,
			ShoesType:      "shoes-test",
			TargetID:       testTargetID,
			CloudID:        "mycloud-uuid",
			ResourceType:   datastore.ResourceTypeNano,
			RepositoryURL:  "https://github.com/octocat/Hello-World",
			RequestWebhook: "{}",
		}
		input.UUID = uuid.FromStringOrNil(fmt.Sprintf(u, i))
		err := testDatastore.CreateRunner(context.Background(), input)
		if err != nil {
			t.Fatalf("failed to create runner: %+v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	recent := time.Now().Add(-10 * time.Second)
	got, err := testDatastore.ListRunnersLogBySince(context.Background(), recent)
	if err != nil {
		t.Fatalf("failed to get runners: %+v", err)
	}
	for i := range got {
		got[i].CreatedAt = time.Time{}
		got[i].UpdatedAt = time.Time{}
	}

	var want []datastore.Runner
	for i := 1; i < 3; i++ {
		r := datastore.Runner{
			UUID:           testRunnerID,
			ShoesType:      "shoes-test",
			TargetID:       testTargetID,
			CloudID:        "mycloud-uuid",
			ResourceType:   datastore.ResourceTypeNano,
			RepositoryURL:  "https://github.com/octocat/Hello-World",
			RequestWebhook: "{}",
		}
		r.UUID = uuid.FromStringOrNil(fmt.Sprintf(u, i))
		want = append(want, r)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestMySQL_GetRunner(t *testing.T) {
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	if err := testDatastore.CreateTarget(context.Background(), datastore.Target{
		UUID:           testTargetID,
		Scope:          testScopeRepo,
		GitHubToken:    testGitHubToken,
		TokenExpiredAt: testTime,
		ResourceType:   datastore.ResourceTypeNano,
	}); err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	if err := testDatastore.CreateRunner(context.Background(), datastore.Runner{
		UUID:           testRunnerID,
		ShoesType:      "shoes-test",
		TargetID:       testTargetID,
		CloudID:        "mycloud-uuid",
		ResourceType:   datastore.ResourceTypeNano,
		RepositoryURL:  "https://github.com/octocat/Hello-World",
		RequestWebhook: "{}",
	}); err != nil {
		t.Fatalf("failed to create runner: %+v", err)
	}

	tests := []struct {
		input uuid.UUID
		want  *datastore.Runner
		err   bool
	}{
		{
			input: testRunnerID,
			want: &datastore.Runner{
				UUID:           testRunnerID,
				ShoesType:      "shoes-test",
				TargetID:       testTargetID,
				CloudID:        "mycloud-uuid",
				ResourceType:   datastore.ResourceTypeNano,
				RepositoryURL:  "https://github.com/octocat/Hello-World",
				RequestWebhook: "{}",
			},
			err: false,
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
		UUID:           testTargetID,
		Scope:          testScopeRepo,
		GitHubToken:    testGitHubToken,
		TokenExpiredAt: testTime,
		ResourceType:   datastore.ResourceTypeNano,
	}); err != nil {
		t.Fatalf("failed to create target: %+v", err)
	}

	if err := testDatastore.CreateRunner(context.Background(), datastore.Runner{
		UUID:           testRunnerID,
		ShoesType:      "shoes-test",
		TargetID:       testTargetID,
		CloudID:        "mycloud-uuid",
		ResourceType:   datastore.ResourceTypeNano,
		RepositoryURL:  "https://github.com/octocat/Hello-World",
		RequestWebhook: "{}",
	}); err != nil {
		t.Fatalf("failed to create runner: %+v", err)
	}

	deleted := datastore.Runner{
		UUID:           testRunnerID,
		ShoesType:      "shoes-test",
		TargetID:       testTargetID,
		CloudID:        "mycloud-uuid",
		ResourceType:   datastore.ResourceTypeNano,
		RepositoryURL:  "https://github.com/octocat/Hello-World",
		RequestWebhook: "{}",
	}

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

		if _, err := getRunningRunnerFromSQL(testDB, test.input); err == nil || errors.Is(err, sql.ErrNoRows) {
			t.Errorf("%s is deleted, but exist in runner_running: %+v", test.input, err)
		}
		if _, err := getDeletedRunnerFromSQL(testDB, test.input); err != nil {
			t.Fatalf("%s is not exist in runners_deleted: %+v", test.input, err)
		}
	}
}

func getRunnerFromSQL(testDB *sqlx.DB, id uuid.UUID) (*datastore.Runner, error) {
	var r datastore.Runner
	query := `SELECT runner_id, shoes_type, ip_address, target_id, cloud_id, created_at, updated_at, resource_type, repository_url, request_webhook, runner_user, provider_url FROM runner_detail WHERE runner_id = ?`
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

func getRunningRunnerFromSQL(testDB *sqlx.DB, id uuid.UUID) (*datastore.Runner, error) {
	var r datastore.Runner
	query := `SELECT detail.runner_id, shoes_type, ip_address, target_id, cloud_id, detail.created_at, updated_at, detail.resource_type, detail.repository_url, detail.request_webhook
FROM runner_detail AS detail JOIN runnesr_running AS running ON detail.runner_id = running.runner_id WHERE detail.runner_id = ?`
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

func getDeletedRunnerFromSQL(testDB *sqlx.DB, id uuid.UUID) (*datastore.Runner, error) {
	var r datastore.Runner
	query := `SELECT detail.runner_id, shoes_type, ip_address, target_id, cloud_id, detail.created_at, updated_at, detail.resource_type, detail.repository_url, detail.request_webhook
FROM runner_detail AS detail JOIN runners_deleted AS deleted ON detail.runner_id = deleted.runner_id WHERE detail.runner_id = ?`
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
