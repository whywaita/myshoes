package gh

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v47/github"
)

func setStubFunctions() {
	GHlistInstallations = func(ctx context.Context, gheDomain string) ([]*github.Installation, error) {
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

	GHlistAppsInstalledRepo = func(ctx context.Context, gheDomain string, installationID int64) ([]*github.Repository, error) {
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
		got, err := IsInstalledGitHubApp(context.Background(), test.input.gheDomain, test.input.scope)
		if !test.err && err != nil {
			t.Fatalf("failed to check GitHub Apps: %+v", err)
		}

		if got != test.want {
			t.Fatalf("want %d, but got %d", test.want, got)
		}
	}
}
