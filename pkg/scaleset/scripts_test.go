package scaleset

import (
	"strings"
	"testing"
)

func TestGetJITSetupScript(t *testing.T) {
	tests := []struct {
		name             string
		encodedJITConfig string
		runnerVersion    string
		runnerUser       string
		runnerBaseDir    string
	}{
		{
			name:             "basic JIT script generation",
			encodedJITConfig: "eyJ0ZXN0IjoidmFsdWUifQ==", // base64 encoded JSON
			runnerVersion:    "v2.311.0",
			runnerUser:       "runner",
			runnerBaseDir:    "/tmp",
		},
		{
			name:             "custom runner user",
			encodedJITConfig: "test-config",
			runnerVersion:    "v2.311.0",
			runnerUser:       "ubuntu",
			runnerBaseDir:    "/home/ubuntu/actions-runner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := GetJITSetupScript(
				tt.encodedJITConfig,
				tt.runnerVersion,
				tt.runnerUser,
				tt.runnerBaseDir,
			)
			if err != nil {
				t.Fatalf("GetJITSetupScript() error = %v", err)
			}

			if script == "" {
				t.Fatal("GetJITSetupScript() returned empty script")
			}

			// Check that script starts with shebang
			if !strings.HasPrefix(script, "#!/bin/bash") {
				t.Error("script should start with #!/bin/bash")
			}

			// Check compression infrastructure
			if !strings.Contains(script, "COMPRESSED_SCRIPT") {
				t.Error("script should contain COMPRESSED_SCRIPT variable")
			}

			if !strings.Contains(script, "base64 -d") {
				t.Error("script should contain base64 decoding")
			}

			if !strings.Contains(script, "gzip -d") {
				t.Error("script should contain gzip decompression")
			}
		})
	}
}

func TestGetJITRawScript(t *testing.T) {
	tests := []struct {
		name             string
		encodedJITConfig string
		runnerVersion    string
		runnerUser       string
		runnerBaseDir    string
		wantContains     []string
		wantNotContains  []string
	}{
		{
			name:             "basic JIT raw script",
			encodedJITConfig: "eyJ0ZXN0IjoidmFsdWUifQ==",
			runnerVersion:    "v2.311.0",
			runnerUser:       "runner",
			runnerBaseDir:    "/tmp",
			wantContains: []string{
				"#!/bin/bash",
				"set -e",
				"RUNNER_VERSION=v2.311.0",
				"RUNNER_USER=runner",
				"RUNNER_BASE_DIRECTORY=/tmp",
				"JIT_CONFIG=eyJ0ZXN0IjoidmFsdWUifQ==",
				"./run.sh --jitconfig",
			},
			wantNotContains: []string{
				"config.sh",        // JIT doesn't use config.sh
				"RUNNER_TOKEN",     // JIT doesn't use registration token
				"RunnerService.js", // JIT doesn't need patch
				"--ephemeral",      // JIT is inherently ephemeral
				"--once",           // JIT doesn't use --once flag
			},
		},
		{
			name:             "custom runner user",
			encodedJITConfig: "test-config",
			runnerVersion:    "v2.311.0",
			runnerUser:       "ubuntu",
			runnerBaseDir:    "/home/ubuntu/actions-runner",
			wantContains: []string{
				"RUNNER_USER=ubuntu",
				"RUNNER_BASE_DIRECTORY=/home/ubuntu/actions-runner",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := getJITRawScript(
				tt.encodedJITConfig,
				tt.runnerVersion,
				tt.runnerUser,
				tt.runnerBaseDir,
			)
			if err != nil {
				t.Fatalf("getJITRawScript() error = %v", err)
			}

			if script == "" {
				t.Fatal("getJITRawScript() returned empty script")
			}

			// Check expected content
			for _, want := range tt.wantContains {
				if !strings.Contains(script, want) {
					t.Errorf("script should contain %q", want)
				}
			}

			// Check excluded content
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(script, notWant) {
					t.Errorf("script should NOT contain %q", notWant)
				}
			}
		})
	}
}

func TestGetJITSetupScriptCompression(t *testing.T) {
	// Test that script is properly compressed and decompressed
	encodedJITConfig := "test-jit-config-value"
	runnerVersion := "v2.311.0"
	runnerUser := "runner"
	runnerBaseDir := "/tmp"

	script, err := GetJITSetupScript(encodedJITConfig, runnerVersion, runnerUser, runnerBaseDir)
	if err != nil {
		t.Fatalf("GetJITSetupScript() error = %v", err)
	}

	// Compressed script should contain base64 decoding
	if !strings.Contains(script, "base64 -d") {
		t.Error("script should contain base64 decoding")
	}

	// Compressed script should contain gzip decompression
	if !strings.Contains(script, "gzip -d") {
		t.Error("script should contain gzip decompression")
	}

	// Should reference COMPRESSED_SCRIPT variable
	if !strings.Contains(script, "COMPRESSED_SCRIPT") {
		t.Error("script should reference COMPRESSED_SCRIPT variable")
	}
}
