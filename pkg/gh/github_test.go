package gh

import "testing"

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
	scope          string
	gheDomain      string
	gheDomainValid bool
}

func TestGetRepositoryURL(t *testing.T) {
	tests := []struct {
		input TestGetRepositoryURLInput
		want  string
		err   error
	}{
		{
			input: TestGetRepositoryURLInput{
				scope:          "org/repo",
				gheDomain:      "",
				gheDomainValid: false,
			},
			want: "https://api.github.com/repos/org/repo",
			err:  nil,
		},
		{
			input: TestGetRepositoryURLInput{
				scope:          "org",
				gheDomain:      "",
				gheDomainValid: false,
			},
			want: "https://api.github.com/orgs/org",
			err:  nil,
		},
		{
			input: TestGetRepositoryURLInput{
				scope:          "org/repo",
				gheDomain:      "github-enterprise.example.com",
				gheDomainValid: true,
			},
			want: "github-enterprise.example.com/api/repos/org/repo",
			err:  nil,
		},
		{
			input: TestGetRepositoryURLInput{
				scope:          "org",
				gheDomain:      "github-enterprise.example.com",
				gheDomainValid: true,
			},
			want: "github-enterprise.example.com/api/orgs/org",
			err:  nil,
		},
		{
			input: TestGetRepositoryURLInput{
				scope:          "org/repo",
				gheDomain:      "https://github-enterprise.example.com/",
				gheDomainValid: true,
			},
			want: "https://github-enterprise.example.com/api/repos/org/repo",
			err:  nil,
		},
	}

	for _, test := range tests {
		got, err := getRepositoryURL(test.input.scope, test.input.gheDomain, test.input.gheDomainValid)
		if err != test.err {
			t.Fatalf("getRepositoryURL want err %+v, but return err %+v", test.err, err)
		}

		if got != test.want {
			t.Fatalf("want %s, but got %s", test.want, got)
		}
	}
}
