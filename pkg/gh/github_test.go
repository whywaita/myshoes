package gh

import (
	"os"
	"testing"

	"github.com/whywaita/myshoes/pkg/config"
)

func TestDetectScope(t *testing.T) {
	tests := []struct {
		input string
		want  Scope
	}{
		{
			input: "org/repo",
			want:  Repository,
		},
		{
			input: "org",
			want:  Organization,
		},
		{
			input: "org/repo/whats",
			want:  Unknown,
		},
		{
			input: "https://github.com/octocat/Spoon-Knife",
			want:  Unknown,
		},
	}

	for _, test := range tests {
		got := DetectScope(test.input)

		if got != test.want {
			t.Fatalf("want %+v, but got %+v", test.want, got)
		}
	}
}

type TestGetRepositoryURLInput struct {
	scope     string
	gheDomain string
}

func TestGetRepositoryURL(t *testing.T) {
	tests := []struct {
		input TestGetRepositoryURLInput
		want  string
		err   error
	}{
		{
			input: TestGetRepositoryURLInput{
				scope:     "org/repo",
				gheDomain: "",
			},
			want: "https://api.github.com/repos/org/repo",
			err:  nil,
		},
		{
			input: TestGetRepositoryURLInput{
				scope:     "org",
				gheDomain: "",
			},
			want: "https://api.github.com/orgs/org",
			err:  nil,
		},
		{
			input: TestGetRepositoryURLInput{
				scope:     "org/repo",
				gheDomain: "https://github-enterprise.example.com",
			},
			want: "https://github-enterprise.example.com/api/v3/repos/org/repo",
			err:  nil,
		},
		{
			input: TestGetRepositoryURLInput{
				scope:     "org",
				gheDomain: "https://github-enterprise.example.com",
			},
			want: "https://github-enterprise.example.com/api/v3/orgs/org",
			err:  nil,
		},
		{
			input: TestGetRepositoryURLInput{
				scope:     "org/repo",
				gheDomain: "https://github-enterprise.example.com/",
			},
			want: "https://github-enterprise.example.com/api/v3/repos/org/repo",
			err:  nil,
		},
		{
			input: TestGetRepositoryURLInput{
				scope:     "org/repo",
				gheDomain: "https://github-enterprise.example.com/github",
			},
			want: "https://github-enterprise.example.com/github/api/v3/repos/org/repo",
			err:  nil,
		},
	}

	for _, test := range tests {
		f := func() {
			if test.input.gheDomain != "" {
				t.Setenv("GITHUB_URL", test.input.gheDomain)
				defer os.Unsetenv("GITHUB_URL")
			}

			config.LoadWithDefault()

			got, err := getRepositoryURL(test.input.scope)
			if err != test.err {
				t.Fatalf("getRepositoryURL want err %+v, but return err %+v", test.err, err)
			}

			if got != test.want {
				t.Fatalf("want %s, but got %s", test.want, got)
			}
		}
		f()
	}
}
