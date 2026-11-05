package gh

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v80/github"
)

func setStubFunctions() {
	GHlistInstallations = func(ctx context.Context) ([]*github.Installation, error) {
		i10 := int64(10)
		i11 := int64(11)
		i12 := int64(12)
		all := "all"
		selected := "selected"
		exampleAll := "example-all"
		exampleSelected := "example-selected"
		exampleSuspented := "example-suspended"

		return []*github.Installation{
			{
				ID: &i10,
				Account: &github.User{
					Login: &exampleAll,
				},
				RepositorySelection: &all,
				SuspendedBy:         nil,
			},
			{
				ID: &i11,
				Account: &github.User{
					Login: &exampleSelected,
				},
				RepositorySelection: &selected,
				SuspendedBy:         nil,
			},
			{
				ID: &i12,
				Account: &github.User{
					Login: &exampleSuspented,
				},
				RepositorySelection: &selected,
				SuspendedAt: &github.Timestamp{
					Time: time.Now(),
				},
			},
		}, nil
	}

	GHlistAppsInstalledRepo = func(ctx context.Context, installationID int64) ([]*github.Repository, error) {
		fullName1 := "example-selected/sample-registered"
		return []*github.Repository{
			{
				FullName: &fullName1,
			},
		}, nil
	}
}

func Test_IsInstalledGitHubApp(t *testing.T) {
	setStubFunctions()

	tests := []struct {
		input struct {
			gheDomain string
			scope     string
		}
		want int64
		err  bool
	}{
		{
			input: struct {
				gheDomain string
				scope     string
			}{gheDomain: "", scope: "example-all"},
			want: 10,
			err:  false,
		},
		{
			input: struct {
				gheDomain string
				scope     string
			}{gheDomain: "", scope: "example-all/sample"},
			want: 10,
			err:  false,
		},
		{
			input: struct {
				gheDomain string
				scope     string
			}{gheDomain: "", scope: "example-selected"},
			want: 11,
			err:  false,
		},
		{
			input: struct {
				gheDomain string
				scope     string
			}{gheDomain: "", scope: "example-selected/sample-registered"},
			want: 11,
			err:  false,
		},
		{
			input: struct {
				gheDomain string
				scope     string
			}{gheDomain: "", scope: "example-selected/sample-not-registered"},
			want: -1,
			err:  true,
		},
		{
			input: struct {
				gheDomain string
				scope     string
			}{gheDomain: "", scope: "example-suspended"},
			want: -1,
			err:  true,
		},
	}

	for _, test := range tests {
		got, err := IsInstalledGitHubApp(context.Background(), test.input.scope)
		if !test.err && err != nil {
			t.Fatalf("failed to check GitHub Apps: %+v", err)
		}

		if got != test.want {
			t.Fatalf("want %d, but got %d", test.want, got)
		}
	}
}

// An installation must not match a scope whose owner only shares a prefix
// with the installation account (e.g. "example" vs "example-other").
func Test_IsInstalledGitHubApp_OwnerExactMatch(t *testing.T) {
	t.Cleanup(setStubFunctions)

	idExample := int64(40)
	idExampleOther := int64(41)
	all := "all"
	selected := "selected"
	example := "example"
	exampleOther := "example-other"
	registered := "example-other/registered"

	GHlistInstallations = func(ctx context.Context) ([]*github.Installation, error) {
		return []*github.Installation{
			{ID: &idExample, Account: &github.User{Login: &example}, RepositorySelection: &all},
			{ID: &idExampleOther, Account: &github.User{Login: &exampleOther}, RepositorySelection: &selected},
		}, nil
	}
	GHlistAppsInstalledRepo = func(ctx context.Context, installationID int64) ([]*github.Repository, error) {
		if installationID != idExampleOther {
			return nil, nil
		}
		return []*github.Repository{{FullName: &registered}}, nil
	}

	tests := []struct {
		scope string
		want  int64
		err   bool
	}{
		{scope: "example", want: idExample},
		{scope: "example/repo", want: idExample},
		{scope: "EXAMPLE/repo", want: idExample},
		{scope: "example-other", want: idExampleOther},
		{scope: "example-other/registered", want: idExampleOther},
		{scope: "example-other/not-registered", want: -1, err: true},
		{scope: "example-another/repo", want: -1, err: true},
		{scope: "exam/repo", want: -1, err: true},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			got, err := IsInstalledGitHubApp(context.Background(), tt.scope)
			if (err != nil) != tt.err {
				t.Fatalf("unexpected error: %+v", err)
			}
			if got != tt.want {
				t.Fatalf("want %d, but got %d", tt.want, got)
			}
		})
	}
}

// GitHub REST API may return null elements in array responses, and
// go-github decodes them into nil pointers without error.
func Test_IsInstalledGitHubApp_NilSafe(t *testing.T) {
	t.Cleanup(setStubFunctions)

	idNoAccount := int64(20)
	idNoLogin := int64(21)
	idSelected := int64(22)
	idNoSelection := int64(23)
	all := "all"
	selected := "selected"
	exampleNil := "example-nil"
	exampleSelected := "example-nil-selected"
	registered := "example-nil-selected/registered"

	GHlistInstallations = func(ctx context.Context) ([]*github.Installation, error) {
		return []*github.Installation{
			nil,
			{ID: &idNoAccount, RepositorySelection: &all},
			{ID: &idNoLogin, Account: &github.User{}, RepositorySelection: &all},
			{Account: &github.User{Login: &exampleNil}, RepositorySelection: &all},
			{ID: &idNoSelection, Account: &github.User{Login: &exampleNil}},
			{ID: &idSelected, Account: &github.User{Login: &exampleSelected}, RepositorySelection: &selected},
		}, nil
	}
	GHlistAppsInstalledRepo = func(ctx context.Context, installationID int64) ([]*github.Repository, error) {
		return []*github.Repository{nil, {}, {FullName: &registered}}, nil
	}

	tests := []struct {
		scope string
		want  int64
		err   bool
	}{
		{scope: "example-nil-selected/registered", want: idSelected},
		{scope: "example-nil-selected/not-registered", want: -1, err: true},
		{scope: "example-nil/repo", want: -1, err: true},
		{scope: "unknown/repo", want: -1, err: true},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			got, err := IsInstalledGitHubApp(context.Background(), tt.scope)
			if (err != nil) != tt.err {
				t.Fatalf("unexpected error: %+v", err)
			}
			if got != tt.want {
				t.Fatalf("want %d, but got %d", tt.want, got)
			}
		})
	}
}
