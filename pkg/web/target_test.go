package web_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/google/go-github/v32/github"

	"github.com/google/go-cmp/cmp"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/internal/testutils"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/web"
)

func parseResponse(resp *http.Response) ([]byte, int) {
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return content, resp.StatusCode
}

func setStubFunctions() {
	web.GHExistGitHubRepositoryFunc = func(scope, gheDomain string, gheDomainValid bool, githubPersonalToken string) error {
		return nil
	}

	web.GHListRunnersFunc = func(ctx context.Context, client *github.Client, owner, repo string) ([]*github.Runner, error) {
		return nil, nil
	}
}

func Test_handleTargetCreate(t *testing.T) {
	testURL := testutils.GetTestURL()
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	tests := []struct {
		input string
		want  *datastore.Target
		err   bool
	}{
		{
			input: `{"scope": "octocat", "ghe_domain": "", "github_personal_token": "secret", "resource_type": "micro", "runner_user": "ubuntu"}`,
			want: &datastore.Target{
				Scope:               "octocat",
				GitHubPersonalToken: "secret",
				ResourceType:        datastore.ResourceTypeMicro,
				RunnerUser: sql.NullString{
					Valid:  true,
					String: "ubuntu",
				},
				Status: datastore.TargetStatusInitialize,
			},
			err: false,
		},
		{
			input: `{"scope": "whywaita/whywaita", "ghe_domain": "github.example.com", "github_personal_token": "secret", "resource_type": "nano", "runner_user": "ubuntu"}`,
			want: &datastore.Target{
				Scope:               "whywaita/whywaita",
				GitHubPersonalToken: "secret",
				GHEDomain: sql.NullString{
					Valid:  true,
					String: "github.example.com",
				},
				ResourceType: datastore.ResourceTypeNano,
				RunnerUser: sql.NullString{
					Valid:  true,
					String: "ubuntu",
				},
				Status: datastore.TargetStatusInitialize,
			},
		},
	}

	for _, test := range tests {
		resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(test.input))
		if !test.err && err != nil {
			t.Fatalf("failed to POST request: %+v", err)
		}
		content, code := parseResponse(resp)
		if code != http.StatusCreated {
			t.Fatalf("must be response statuscode is 201, but got %d", code)
		}

		var gotContent datastore.Target
		if err := json.Unmarshal(content, &gotContent); err != nil {
			t.Fatalf("failed to unmarshal resoponse content: %+v", err)
		}

		u := gotContent.UUID
		gotContent.UUID = uuid.UUID{}
		gotContent.GitHubPersonalToken = "secret" // http response hasn't a token
		gotContent.CreatedAt = time.Time{}
		gotContent.UpdatedAt = time.Time{}

		if diff := cmp.Diff(test.want, &gotContent); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}

		got, err := testDatastore.GetTarget(context.Background(), u)
		if err != nil {
			t.Fatalf("failed to retrieve target from datastore: %+v", err)
		}
		got.UUID = uuid.UUID{}
		got.CreatedAt = time.Time{}
		got.UpdatedAt = time.Time{}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func Test_handleTargetCreate_alreadyRegistered(t *testing.T) {
	testURL := testutils.GetTestURL()
	_, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	input := `{"scope": "octocat", "ghe_domain": "", "github_personal_token": "secret", "resource_type": "micro", "runner_user": "ubuntu"}`

	// first create
	resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	_, code := parseResponse(resp)
	if code != http.StatusCreated {
		t.Fatalf("must be response statuscode is 201, but got %d", code)
	}

	// second create
	resp, err = http.Post(testURL+"/target", "application/json", bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	_, code = parseResponse(resp)
	if code != http.StatusBadRequest {
		t.Fatalf("must be response statuscode is 400, but got %d", code)
	}
}

func Test_handleTargetCreate_recreated(t *testing.T) {
	testURL := testutils.GetTestURL()
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	input := `{"scope": "octocat", "ghe_domain": "", "github_personal_token": "secret", "resource_type": "micro", "runner_user": "ubuntu"}`

	// first create
	resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	content, code := parseResponse(resp)
	if code != http.StatusCreated {
		t.Fatalf("must be response statuscode is 201, but got %d: %+v", code, string(content))
	}
	var gotContent datastore.Target
	if err := json.Unmarshal(content, &gotContent); err != nil {
		t.Fatalf("failed to unmarshal resoponse content: %+v", err)
	}

	u := gotContent.UUID

	// first delete
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/target/%s", testURL, u.String()), nil)
	if err != nil {
		t.Fatalf("failed to create request: %+v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	_, code = parseResponse(resp)
	if code != http.StatusNoContent {
		t.Fatalf("must be response statuscode is 204, but got %d: %+v", code, string(content))
	}

	// second create
	resp, err = http.Post(testURL+"/target", "application/json", bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	content, code = parseResponse(resp)
	if code != http.StatusCreated {
		t.Fatalf("must be response statuscode is 201, but got %d: %+v", code, string(content))
	}

	got, err := testDatastore.GetTarget(context.Background(), u)
	if err != nil {
		t.Fatalf("failed to get created target: %+v", err)
	}
	if got.Status != datastore.TargetStatusInitialize {
		t.Fatalf("must be status is initialize when recreated")
	}
}

func Test_handleTargetList(t *testing.T) {
	testURL := testutils.GetTestURL()
	_, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	for _, rt := range []string{"nano", "micro"} {
		target := fmt.Sprintf(`{"scope": "repo%s", "github_personal_token": "secret", "resource_type": "%s", "runner_user": "ubuntu"}`,
			rt, rt)
		resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(target))
		if err != nil {
			t.Fatalf("failed to POST request: %+v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("must be response statuscode is 201, but got %d", resp.StatusCode)
		}
	}

	tests := []struct {
		input interface{}
		want  *[]datastore.Target
		err   bool
	}{
		{
			input: nil,
			want: &[]datastore.Target{
				{
					Scope:               "reponano",
					GitHubPersonalToken: "",
					ResourceType:        datastore.ResourceTypeNano,
					RunnerUser: sql.NullString{
						Valid:  true,
						String: "ubuntu",
					},
					Status: datastore.TargetStatusInitialize,
				},
				{
					Scope:               "repomicro",
					GitHubPersonalToken: "",
					ResourceType:        datastore.ResourceTypeMicro,
					RunnerUser: sql.NullString{
						Valid:  true,
						String: "ubuntu",
					},
					Status: datastore.TargetStatusInitialize,
				},
			},
		},
	}

	for _, test := range tests {
		resp, err := http.Get(testURL + "/target")
		if !test.err && err != nil {
			t.Fatalf("failed to POST request: %+v", err)
		}
		content, code := parseResponse(resp)
		if code != http.StatusOK {
			t.Fatalf("must be response statuscode is 201, but got %d: %+v", code, string(content))
		}

		var gotContents []datastore.Target
		if err := json.Unmarshal(content, &gotContents); err != nil {
			t.Fatalf("failed to unmarshal resoponse content: %+v", err)
		}

		sort.Slice(gotContents, func(i, j int) bool {
			return gotContents[i].ResourceType < gotContents[j].ResourceType
		})

		for i := range gotContents {
			gotContents[i].UUID = uuid.UUID{}
			gotContents[i].CreatedAt = time.Time{}
			gotContents[i].UpdatedAt = time.Time{}
		}

		if diff := cmp.Diff(test.want, &gotContents); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func Test_handleTargetRead(t *testing.T) {
	testURL := testutils.GetTestURL()
	_, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	target := `{"scope": "repo", "github_personal_token": "secret", "resource_type": "micro", "runner_user": "ubuntu"}`

	resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(target))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	content, statusCode := parseResponse(resp)
	if statusCode != http.StatusCreated {
		t.Fatalf("must be response statuscode is 201, but got %d: %+v", resp.StatusCode, string(content))
	}
	var respTarget datastore.Target
	if err := json.Unmarshal(content, &respTarget); err != nil {
		t.Fatalf("failed to unmarshal response JSON: %+v", err)
	}
	targetUUID := respTarget.UUID

	tests := []struct {
		input uuid.UUID
		want  *datastore.Target
		err   bool
	}{
		{
			input: targetUUID,
			want: &datastore.Target{
				UUID:                targetUUID,
				Scope:               "repo",
				GitHubPersonalToken: "",
				ResourceType:        datastore.ResourceTypeMicro,
				RunnerUser: sql.NullString{
					Valid:  true,
					String: "ubuntu",
				},
				Status: datastore.TargetStatusInitialize,
			},
		},
	}

	for _, test := range tests {
		resp, err := http.Get(fmt.Sprintf("%s/target/%s", testURL, test.input))
		if !test.err && err != nil {
			t.Fatalf("failed to POST request: %+v", err)
		}
		content, code := parseResponse(resp)
		if code != http.StatusOK {
			t.Fatalf("must be response statuscode is 201, but got %d: %+v", code, string(content))
		}

		var got datastore.Target
		if err := json.Unmarshal(content, &got); err != nil {
			t.Fatalf("failed to unmarshal resoponse content: %+v", err)
		}

		got.CreatedAt = time.Time{}
		got.UpdatedAt = time.Time{}

		if diff := cmp.Diff(test.want, &got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func Test_handleTargetDelete(t *testing.T) {
	testURL := testutils.GetTestURL()
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	target := `{"scope": "repo", "github_personal_token": "secret", "resource_type": "micro", "runner_user": "ubuntu"}`

	resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(target))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	content, statusCode := parseResponse(resp)
	if statusCode != http.StatusCreated {
		t.Fatalf("must be response statuscode is 201, but got %d: %+v", resp.StatusCode, string(content))
	}
	var respTarget datastore.Target
	if err := json.Unmarshal(content, &respTarget); err != nil {
		t.Fatalf("failed to unmarshal response JSON: %+v", err)
	}
	targetUUID := respTarget.UUID

	tests := []struct {
		input uuid.UUID
		want  *datastore.Target
		err   bool
	}{
		{
			input: targetUUID,
			want: &datastore.Target{
				UUID:                targetUUID,
				Scope:               "repo",
				GitHubPersonalToken: "secret",
				ResourceType:        datastore.ResourceTypeMicro,
				RunnerUser: sql.NullString{
					Valid:  true,
					String: "ubuntu",
				},
				Status: datastore.TargetStatusDeleted,
			},
		},
	}

	for _, test := range tests {
		client := &http.Client{}

		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/target/%s", testURL, test.input), nil)
		if err != nil {
			t.Fatalf("failed to create request: %+v", err)
		}

		resp, err := client.Do(req)
		if !test.err && err != nil {
			t.Fatalf("failed to POST request: %+v", err)
		}
		content, code := parseResponse(resp)
		if code != http.StatusNoContent {
			t.Fatalf("must be response statuscode is 204, but got %d: %+v", code, string(content))
		}

		got, err := testDatastore.GetTarget(context.Background(), test.input)
		if err != nil {
			t.Fatalf("failed to get target from datastore: %+v", err)
		}

		got.CreatedAt = time.Time{}
		got.UpdatedAt = time.Time{}

		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}
