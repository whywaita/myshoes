package gh

import (
	"testing"

	"github.com/google/go-github/v80/github"
)

func Test_getRunnerVersion(t *testing.T) {
	linux := "linux"
	osx := "osx"
	x64 := "x64"
	filename := "actions-runner-linux-x64-2.330.0.tar.gz"

	tests := []struct {
		name         string
		applications []*github.RunnerApplicationDownload
		want         string
		err          bool
	}{
		{
			name: "found",
			applications: []*github.RunnerApplicationDownload{
				{OS: &osx, Architecture: &x64},
				{OS: &linux, Architecture: &x64, Filename: &filename},
			},
			want: "v2.330.0",
		},
		{
			name: "nil elements and fields are skipped",
			applications: []*github.RunnerApplicationDownload{
				nil,
				{},
				{OS: &linux},
				{OS: &linux, Architecture: &x64},
				{OS: &linux, Architecture: &x64, Filename: &filename},
			},
			want: "v2.330.0",
		},
		{
			name:         "not found",
			applications: []*github.RunnerApplicationDownload{nil, {OS: &osx, Architecture: &x64}},
			err:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRunnerVersion(tt.applications)
			if (err != nil) != tt.err {
				t.Fatalf("unexpected error: %+v", err)
			}
			if got != tt.want {
				t.Fatalf("want %s, but got %s", tt.want, got)
			}
		})
	}
}
