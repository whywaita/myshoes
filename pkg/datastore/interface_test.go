package datastore_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/whywaita/myshoes/pkg/datastore"
)

func TestTarget_RepoURL(t *testing.T) {
	tests := []struct {
		input datastore.Target
		want  string
		err   bool
	}{
		{
			input: datastore.Target{
				GHEDomain: sql.NullString{
					Valid:  false,
					String: "",
				},
				Scope: "example/octocat",
			},
			want: "https://github.com/example/octocat",
			err:  false,
		},
		{
			input: datastore.Target{
				GHEDomain: sql.NullString{
					Valid:  true,
					String: "https://github-enterprise.example.com",
				},
				Scope: "example",
			},
			want: "https://github-enterprise.example.com/example",
			err:  false,
		},
	}

	for _, test := range tests {
		got := test.input.RepoURL()

		if !strings.EqualFold(got, test.want) {
			t.Fatalf("want %s, but got %s", test.want, got)
		}
	}
}
