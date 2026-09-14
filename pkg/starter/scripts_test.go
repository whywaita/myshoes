package starter

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestRunnerServiceNodeRuntime(t *testing.T) {
	var rendered bytes.Buffer
	tmpl, err := template.New("setup").Parse(templateCreateLatestRunnerOnce)
	if err != nil {
		t.Fatal(err)
	}
	if err := tmpl.Execute(&rendered, templateCreateLatestRunnerOnceValue{}); err != nil {
		t.Fatal(err)
	}

	// Execute the actual heredoc so expansion happens at setup time, then run
	// the generated service script against the runtimes bundled with a runner.
	const start = "cat << EOF > ./bin/runsvc.sh\n"
	_, patch, ok := strings.Cut(rendered.String(), start)
	if !ok {
		t.Fatal("service script heredoc not found")
	}
	body, _, ok := strings.Cut(patch, "\nEOF\n")
	if !ok {
		t.Fatal("service script heredoc terminator not found")
	}

	for _, tc := range []struct {
		name     string
		runtimes []string
		want     string
	}{
		{"prefer node24", []string{"node24", "node20", "node16"}, "node24"},
		{"node24 only", []string{"node24"}, "node24"},
		{"fallback to node20", []string{"node20", "node16"}, "node20"},
		{"node20 only", []string{"node20"}, "node20"},
		{"fallback to node16", []string{"node16"}, "node16"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
				t.Fatal(err)
			}
			for _, runtime := range tc.runtimes {
				path := filepath.Join(dir, "externals", runtime, "bin", "node")
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				stub := "#!/bin/bash\nprintf '%s\\n' '" + runtime + "' \"$@\"\n"
				if err := os.WriteFile(path, []byte(stub), 0755); err != nil {
					t.Fatal(err)
				}
			}
			setup := exec.Command("bash", "-c", start+body+"\nEOF\n")
			setup.Dir = dir
			if output, err := setup.CombinedOutput(); err != nil {
				t.Fatalf("generate service script: %v\n%s", err, output)
			}
			service := exec.Command("bash", "./bin/runsvc.sh", "--once")
			service.Dir = dir
			output, err := service.CombinedOutput()
			if err != nil {
				t.Fatalf("run service script: %v\n%s", err, output)
			}
			want := tc.want + "\n./bin/RunnerService.js\n--once\n"
			if string(output) != want {
				t.Fatalf("service invocation = %q, want %q", output, want)
			}
		})
	}
}
