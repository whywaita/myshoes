package scaleset

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"text/template"
)

// GetJITSetupScript generates a simplified setup script for JIT (Just-In-Time) runner
// JIT config eliminates the need for registration token, config.sh, and RunnerService.js patch
func GetJITSetupScript(encodedJITConfig, runnerVersion, runnerUser, runnerBaseDir string) (string, error) {
	rawScript, err := getJITRawScript(encodedJITConfig, runnerVersion, runnerUser, runnerBaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to get raw JIT script: %w", err)
	}

	var compressedScript bytes.Buffer
	gz := gzip.NewWriter(&compressedScript)
	if _, err := gz.Write([]byte(rawScript)); err != nil {
		return "", fmt.Errorf("failed to compress gzip: %w", err)
	}
	if err := gz.Flush(); err != nil {
		return "", fmt.Errorf("failed to flush gzip: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("failed to close gzip: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(compressedScript.Bytes())

	v := templateCompressedScriptValue{
		CompressedScript:    encoded,
		RunnerBaseDirectory: runnerBaseDir,
	}

	t, err := template.New("templateCompressedScript").Parse(templateCompressedScript)
	if err != nil {
		return "", fmt.Errorf("failed to create template: %w", err)
	}
	var buff bytes.Buffer
	if err := t.Execute(&buff, v); err != nil {
		return "", fmt.Errorf("failed to execute compressed script: %w", err)
	}
	return buff.String(), nil
}

func getJITRawScript(encodedJITConfig, runnerVersion, runnerUser, runnerBaseDir string) (string, error) {
	v := templateJITRunnerValue{
		EncodedJITConfig:    encodedJITConfig,
		RunnerVersion:       runnerVersion,
		RunnerUser:          runnerUser,
		RunnerBaseDirectory: runnerBaseDir,
	}

	t, err := template.New("templateJITRunner").Parse(templateJITRunner)
	if err != nil {
		return "", fmt.Errorf("failed to create template: %w", err)
	}
	var buff bytes.Buffer
	if err := t.Execute(&buff, v); err != nil {
		return "", fmt.Errorf("failed to execute JIT script: %w", err)
	}
	return buff.String(), nil
}

type templateCompressedScriptValue struct {
	CompressedScript    string
	RunnerBaseDirectory string
}

const templateCompressedScript = `#!/bin/bash

set -e

# main script compressed base64 and gzip
export COMPRESSED_SCRIPT={{.CompressedScript}}
export MAIN_SCRIPT_PATH={{.RunnerBaseDirectory}}/main.sh

echo ${COMPRESSED_SCRIPT} | base64 -d | gzip -d > ${MAIN_SCRIPT_PATH}

chmod +x ${MAIN_SCRIPT_PATH}
bash -c ${MAIN_SCRIPT_PATH}`

type templateJITRunnerValue struct {
	EncodedJITConfig    string
	RunnerVersion       string
	RunnerUser          string
	RunnerBaseDirectory string
}

// templateJITRunner is a simplified script template for JIT runner
// Unlike traditional registration, JIT runner only needs to:
// 1. Download runner binary
// 2. Run with --jitconfig flag
const templateJITRunner = `#!/bin/bash

set -e

RUNNER_VERSION={{.RunnerVersion}}
RUNNER_USER={{.RunnerUser}}
RUNNER_BASE_DIRECTORY={{.RunnerBaseDirectory}}
JIT_CONFIG={{.EncodedJITConfig}}

sudo_prefix=""
if [ $(id -u) -eq 0 ]; then
	sudo_prefix="sudo -E -u ${RUNNER_USER} "
fi

echo "Configuring JIT runner"

#---------------------------------------
# Validate Environment
#---------------------------------------
runner_plat=linux
[ ! -z "$(which sw_vers)" ] && runner_plat=osx;

function fatal()
{
	echo "error: $1" >&2
	exit 1
}

function configure_environment()
{
	export HOME="/home/${RUNNER_USER}"
	if [ "${runner_plat}" = "osx" ]; then
		export HOME="/Users/${RUNNER_USER}"
	fi
}

function get_runner_file_name()
{
	runner_version=$1
	runner_plat=$2

	trimmed_runner_version=$(echo ${RUNNER_VERSION:1})

	if [ "${runner_plat}" = "linux" ]; then
		echo "actions-runner-${runner_plat}-x64-${trimmed_runner_version}.tar.gz"
	fi

	if [ "${runner_plat}" = "osx" ]; then
		runner_arch=x64
		[ "$(uname -m)" = "arm64" ] && runner_arch=arm64;
		echo "actions-runner-${runner_plat}-${runner_arch}-${trimmed_runner_version}.tar.gz"
	fi
}

function download_runner()
{
	runner_version=$1
	runner_file=$2

	cd ${RUNNER_BASE_DIRECTORY}

	echo "Downloading ${runner_file}"
	curl -sSL -o ${runner_file} https://github.com/actions/runner/releases/download/${runner_version}/${runner_file}

	ls -la *.tar.gz
	tar xzf ./${runner_file}
}

configure_environment

runner_file=$(get_runner_file_name ${RUNNER_VERSION} ${runner_plat})

cd ${RUNNER_BASE_DIRECTORY}

# Check if runner binary already exists
if [ ! -f "./run.sh" ]; then
	download_runner ${RUNNER_VERSION} ${runner_file}
fi

#---------------------------------------
# Run with JIT config
#---------------------------------------
echo "Starting runner with JIT config"
${sudo_prefix}./run.sh --jitconfig "${JIT_CONFIG}"
`
