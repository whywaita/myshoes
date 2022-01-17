package web_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v35/github"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/internal/testutils"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/web"
)

var testInstallationID = int64(100000000)
var testGitHubAppToken = "secret-app-token"
var testTime = time.Date(2037, 9, 3, 0, 0, 0, 0, time.UTC)

func parseResponse(resp *http.Response) ([]byte, int) {
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return content, resp.StatusCode
}

func setStubFunctions() {
	web.GHExistGitHubRepositoryFunc = func(scope, gheDomain string, githubPersonalToken string) error {
		return nil
	}

	web.GHListRunnersFunc = func(ctx context.Context, client *github.Client, owner, repo string) ([]*github.Runner, error) {
		return nil, nil
	}

	web.GHIsInstalledGitHubApp = func(ctx context.Context, gheDomain, inputScope string) (int64, error) {
		return testInstallationID, nil
	}

	web.GHGenerateGitHubAppsToken = func(ctx context.Context, clientInstallation *github.Client, installationID int64, scope string) (string, *time.Time, error) {
		return testGitHubAppToken, &testTime, nil
	}

	web.GHNewClientApps = func(gheDomain string) (*github.Client, error) {
		return &github.Client{}, nil
	}
}

func Test_handleTargetCreate(t *testing.T) {
	testURL := testutils.GetTestURL()
	_, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	tests := []struct {
		input string
		want  *web.UserTarget
		err   bool
	}{
		{
			input: `{"scope": "octocat", "ghe_domain": "", "resource_type": "micro", "runner_user": "ubuntu"}`,
			want: &web.UserTarget{
				Scope:          "octocat",
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeMicro.String(),
				RunnerUser:     "ubuntu",
				Status:         datastore.TargetStatusActive,
			},
			err: false,
		},
		{
			input: `{"scope": "whywaita/whywaita", "ghe_domain": "github.example.com", "resource_type": "nano", "runner_user": "ubuntu"}`,
			want: &web.UserTarget{
				Scope:          "whywaita/whywaita",
				TokenExpiredAt: testTime,
				GHEDomain:      "github.example.com",
				ResourceType:   datastore.ResourceTypeNano.String(),
				RunnerUser:     "ubuntu",
				Status:         datastore.TargetStatusActive,
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

		var gotContent web.UserTarget
		if err := json.Unmarshal(content, &gotContent); err != nil {
			t.Fatalf("failed to unmarshal resoponse content: %+v", err)
		}

		gotContent.UUID = uuid.UUID{}
		gotContent.CreatedAt = time.Time{}
		gotContent.UpdatedAt = time.Time{}

		if diff := cmp.Diff(test.want, &gotContent); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func Test_handleTargetCreate_alreadyRegistered(t *testing.T) {
	testURL := testutils.GetTestURL()
	_, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	input := `{"scope": "octocat", "ghe_domain": "", "resource_type": "micro", "runner_user": "ubuntu"}`

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

	input := `{"scope": "octocat", "ghe_domain": "", "resource_type": "micro", "runner_user": "ubuntu"}`

	// first create
	resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	content, code := parseResponse(resp)
	if code != http.StatusCreated {
		t.Fatalf("must be response statuscode is 201, but got %d: %+v", code, string(content))
	}
	var gotContent web.UserTarget
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
	if got.Status != datastore.TargetStatusActive {
		t.Fatalf("must be status is active when recreated")
	}
}

func Test_handleTargetCreate_recreated_update(t *testing.T) {
	testURL := testutils.GetTestURL()
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	input := `{"scope": "octocat", "ghe_domain": "", "resource_type": "micro", "runner_user": "ubuntu"}`

	// first create
	resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	content, code := parseResponse(resp)
	if code != http.StatusCreated {
		t.Fatalf("must be response statuscode is 201, but got %d: %+v", code, string(content))
	}
	var gotContent web.UserTarget
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
	secondInput := `{"scope": "octocat", "ghe_domain": "", "resource_type": "micro", "runner_user": "ubuntu", "runner_version": "v0.000.1"}`

	resp, err = http.Post(testURL+"/target", "application/json", bytes.NewBufferString(secondInput))
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
	if got.Status != datastore.TargetStatusActive {
		t.Fatalf("must be status is active when recreated")
	}
	if got.RunnerVersion.String != "v0.000.1" {
		t.Fatalf("must be update runner_version in second createm")
	}
}

func Test_handleTargetList(t *testing.T) {
	testURL := testutils.GetTestURL()
	_, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	for _, rt := range []string{"nano", "micro"} {
		target := fmt.Sprintf(`{"scope": "repo%s", "resource_type": "%s", "runner_user": "ubuntu"}`,
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
		want  *[]web.UserTarget
		err   bool
	}{
		{
			input: nil,
			want: &[]web.UserTarget{
				{
					Scope:          "reponano",
					TokenExpiredAt: testTime,
					ResourceType:   datastore.ResourceTypeNano.String(),
					RunnerUser:     "ubuntu",
					Status:         datastore.TargetStatusActive,
				},
				{
					Scope:          "repomicro",
					TokenExpiredAt: testTime,
					ResourceType:   datastore.ResourceTypeMicro.String(),
					RunnerUser:     "ubuntu",
					Status:         datastore.TargetStatusActive,
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

		var gotContents []web.UserTarget
		if err := json.Unmarshal(content, &gotContents); err != nil {
			t.Fatalf("failed to unmarshal resoponse content: %+v", err)
		}

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

	target := `{"scope": "repo", "resource_type": "micro", "runner_user": "ubuntu"}`

	resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(target))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	content, statusCode := parseResponse(resp)
	if statusCode != http.StatusCreated {
		t.Fatalf("must be response statuscode is 201, but got %d: %+v", resp.StatusCode, string(content))
	}
	var respTarget web.UserTarget
	if err := json.Unmarshal(content, &respTarget); err != nil {
		t.Fatalf("failed to unmarshal response JSON: %+v", err)
	}
	targetUUID := respTarget.UUID

	tests := []struct {
		input uuid.UUID
		want  *web.UserTarget
		err   bool
	}{
		{
			input: targetUUID,
			want: &web.UserTarget{
				UUID:           targetUUID,
				Scope:          "repo",
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeMicro.String(),
				RunnerUser:     "ubuntu",
				Status:         datastore.TargetStatusActive,
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

		var got web.UserTarget
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

func Test_handleTargetUpdate(t *testing.T) {
	testURL := testutils.GetTestURL()
	_, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	tests := []struct {
		input string
		want  *web.UserTarget
		err   bool
	}{
		{ // Update a few values
			input: `{"scope": "repo", "resource_type": "nano", "runner_user": "ubuntu"}`,
			want: &web.UserTarget{
				UUID:           uuid.UUID{},
				Scope:          "repo",
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeNano.String(),
				RunnerUser:     "ubuntu",
				RunnerVersion:  "",
				ProviderURL:    "https://example.com/default-shoes",
				Status:         datastore.TargetStatusActive,
			},
		},
		{ // Update all values
			input: `{"scope": "repo", "resource_type": "micro", "runner_user": "super-user", "runner_version": "v9.999.9", "provider_url": "https://example.com/shoes-provider"}`,
			want: &web.UserTarget{
				UUID:           uuid.UUID{},
				Scope:          "repo",
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeMicro.String(),
				RunnerUser:     "super-user",
				RunnerVersion:  "v9.999.9",
				ProviderURL:    "https://example.com/shoes-provider",
				Status:         datastore.TargetStatusActive,
			},
		},
		{ // Update value only one, other value is not update
			input: `{"scope": "repo", "resource_type": "nano"}`,
			want: &web.UserTarget{
				UUID:           uuid.UUID{},
				Scope:          "repo",
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeNano.String(),
				RunnerUser:     "ubuntu",
				RunnerVersion:  "",
				ProviderURL:    "https://example.com/default-shoes",
				Status:         datastore.TargetStatusActive,
			},
		},
		{ // Remove provider_url, Set blank
			input: `{"scope": "repo", "resource_type": "nano" ,"provider_url": ""}`,
			want: &web.UserTarget{
				UUID:           uuid.UUID{},
				Scope:          "repo",
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeNano.String(),
				RunnerUser:     "ubuntu",
				RunnerVersion:  "",
				ProviderURL:    "",
				Status:         datastore.TargetStatusActive,
			},
		},
	}

	for _, test := range tests {
		target := `{"scope": "repo", "resource_type": "micro", "runner_user": "ubuntu", "provider_url": "https://example.com/default-shoes"}`
		respCreate, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(target))
		if err != nil {
			t.Fatalf("failed to POST request: %+v", err)
		}
		contentCreate, statusCode := parseResponse(respCreate)
		if statusCode != http.StatusCreated {
			t.Fatalf("must be response statuscode is 201, but got %d: %+v", respCreate.StatusCode, string(contentCreate))
		}
		var respTarget web.UserTarget
		if err := json.Unmarshal(contentCreate, &respTarget); err != nil {
			t.Fatalf("failed to unmarshal response JSON: %+v", err)
		}
		targetUUID := respTarget.UUID

		resp, err := http.Post(fmt.Sprintf("%s/target/%s", testURL, targetUUID.String()), "application/json", bytes.NewBufferString(test.input))
		if !test.err && err != nil {
			t.Fatalf("failed to POST request: %+v", err)
		}
		content, code := parseResponse(resp)
		if code != http.StatusOK {
			t.Fatalf("must be response statuscode is 201, but got %d: %+v", code, string(content))
		}

		var got web.UserTarget
		if err := json.Unmarshal(content, &got); err != nil {
			t.Fatalf("failed to unmarshal resoponse content: %+v", err)
		}

		got.UUID = uuid.UUID{}
		got.CreatedAt = time.Time{}
		got.UpdatedAt = time.Time{}

		if diff := cmp.Diff(test.want, &got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}

		teardown()
	}
}

func Test_handleTargetDelete(t *testing.T) {
	testURL := testutils.GetTestURL()
	testDatastore, teardown := testutils.GetTestDatastore()
	defer teardown()

	setStubFunctions()

	target := `{"scope": "repo", "resource_type": "micro", "runner_user": "ubuntu"}`

	resp, err := http.Post(testURL+"/target", "application/json", bytes.NewBufferString(target))
	if err != nil {
		t.Fatalf("failed to POST request: %+v", err)
	}
	content, statusCode := parseResponse(resp)
	if statusCode != http.StatusCreated {
		t.Fatalf("must be response statuscode is 201, but got %d: %+v", resp.StatusCode, string(content))
	}
	var respTarget web.UserTarget
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
				UUID:           targetUUID,
				Scope:          "repo",
				GitHubToken:    testGitHubAppToken,
				TokenExpiredAt: testTime,
				ResourceType:   datastore.ResourceTypeMicro,
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
