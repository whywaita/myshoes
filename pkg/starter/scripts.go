package starter

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed" // TODO:
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"

	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/runner"
)

//go:embed scripts/RunnerService.js
var runnerService string

func getPatchedFiles() (string, error) {
	return runnerService, nil
}

func (s *Starter) getSetupScript(ctx context.Context, targetScope, runnerName string) (string, error) {
	rawScript, err := s.getSetupRawScript(ctx, targetScope, runnerName)
	if err != nil {
		return "", fmt.Errorf("failed to get raw setup scripts: %w", err)
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

	return fmt.Sprintf(templateCompressedScript, encoded), nil
}

func (s *Starter) getSetupRawScript(ctx context.Context, targetScope, runnerName string) (string, error) {
	runnerUser := config.Config.RunnerUser
	githubURL := config.Config.GitHubURL

	targetRunnerVersion := s.runnerVersion
	if strings.EqualFold(s.runnerVersion, "latest") {
		latestVersion, err := gh.GetLatestRunnerVersion(ctx, targetScope)
		if err != nil {
			return "", fmt.Errorf("failed to get latest version of actions/runner: %w", err)
		}
		targetRunnerVersion = latestVersion
	}

	runnerVersion, runnerTemporaryMode, err := runner.GetRunnerTemporaryMode(targetRunnerVersion)
	if err != nil {
		return "", fmt.Errorf("failed to get runner version: %w", err)
	}

	runnerServiceJs, err := getPatchedFiles()
	if err != nil {
		return "", fmt.Errorf("failed to get patched files: %w", err)
	}

	installationID, err := gh.IsInstalledGitHubApp(ctx, targetScope)
	if err != nil {
		return "", fmt.Errorf("failed to get installlation id: %w", err)
	}
	token, err := gh.GetRunnerRegistrationToken(ctx, installationID, targetScope)
	if err != nil {
		return "", fmt.Errorf("failed to generate runner register token: %w", err)
	}

	var labels []string
	if githubURL != "" && githubURL != "https://github.com" {
		labels = append(labels, "dependabot")
	}

	v := templateCreateLatestRunnerOnceValue{
		Scope:                   targetScope,
		GHEDomain:               config.Config.GitHubURL,
		RunnerRegistrationToken: token,
		RunnerName:              runnerName,
		RunnerUser:              runnerUser,
		RunnerVersion:           runnerVersion,
		RunnerServiceJS:         runnerServiceJs,
		RunnerArg:               runnerTemporaryMode.StringFlag(),
		AdditionalLabels:        labelsToOneLine(labels),
	}

	t, err := template.New("templateCreateLatestRunnerOnce").Parse(templateCreateLatestRunnerOnce)
	if err != nil {
		return "", fmt.Errorf("failed to create template")
	}
	var buff bytes.Buffer
	if err := t.Execute(&buff, v); err != nil {
		return "", fmt.Errorf("failed to execute scripts: %w", err)
	}
	return buff.String(), nil
}

func labelsToOneLine(labels []string) string {
	if len(labels) == 0 {
		return ""
	}

	return fmt.Sprintf(",%s", strings.Join(labels, ","))
}

const templateCompressedScript = `#!/bin/bash

set -e

# main script compressed base64 and gzip
export COMPRESSED_SCRIPT=%s
export MAIN_SCRIPT_PATH=/tmp/main.sh

echo ${COMPRESSED_SCRIPT} | base64 -d | gzip -d > ${MAIN_SCRIPT_PATH}

chmod +x ${MAIN_SCRIPT_PATH}
bash -c ${MAIN_SCRIPT_PATH}`

type templateCreateLatestRunnerOnceValue struct {
	Scope                   string
	GHEDomain               string
	RunnerRegistrationToken string
	RunnerName              string
	RunnerUser              string
	RunnerVersion           string
	RunnerServiceJS         string
	RunnerArg               string
	AdditionalLabels        string
}

// templateCreateLatestRunnerOnce is script template of setup runner.
// need to set runnerUser if execute using root permission. (for example, use cloud-init)
// original script: https://github.com/actions/runner/blob/80bf68db812beb298b7534012b261e6f222e004a/scripts/create-latest-svc.sh
const templateCreateLatestRunnerOnce = `#!/bin/bash

set -e

runner_scope={{.Scope}}
ghe_hostname={{.GHEDomain}}
runner_name={{.RunnerName}}
RUNNER_TOKEN={{.RunnerRegistrationToken}}
RUNNER_USER={{.RunnerUser}}
RUNNER_VERSION={{.RunnerVersion}}
RUNNER_BASE_DIRECTORY=/tmp  # /tmp is path of all user writable.

sudo_prefix=""
if [ $(id -u) -eq 0 ]; then  # if root
sudo_prefix="sudo -E -u ${RUNNER_USER} "
fi

echo "Configuring runner @ ${runner_scope}"

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

function install_jq()
{
    echo "jq is not installed, will be install jq."
    if [ -e /etc/debian_version ] || [ -e /etc/debian_release ]; then
        sudo apt-get update -y -qq
        sudo apt-get install -y jq
    elif [ -e /etc/redhat-release ]; then
        sudo yum install -y jq
    fi

	if [ "${runner_plat}" = "osx" ]; then
		brew install jq
	fi
}

function install_docker()
{
	echo "docker is not installed, will be install docker."
	if [ -e /etc/debian_version ] || [ -e /etc/debian_release ]; then
		sudo apt-get update -y -qq
		sudo apt-get install -y docker.io
	fi

	if [ "${runner_plat}" = "osx" ]; then
		echo "No install in macOS, It is same that GitHub-hosted" 
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

    runner_url="https://github.com/actions/runner/releases/download/${runner_version}/${runner_file}"

    echo "Downloading ${runner_version} for ${runner_plat} ..."
    echo $runner_url

    curl -O -L ${runner_url}

    ls -la *.tar.gz
}

function extract_runner()
{
	runner_file=$1
	runner_user=$2

	echo "Extracting ${runner_file} to ./runner"

	tar xzf "./${runner_file}" -C runner

	# export of pass
	if [ $(id -u) -eq 0 ]; then
	chown -R ${runner_user} ./runner
	fi
}

if [ -z "${runner_scope}" ]; then fatal "supply scope as argument 1"; fi

which curl || fatal "curl required.  Please install in PATH with apt-get, brew, etc"
which jq || install_jq
which jq || fatal "jq required.  Please install in PATH with apt-get, brew, etc"
which docker || install_docker

configure_environment

cd ${RUNNER_BASE_DIRECTORY}
${sudo_prefix}mkdir -p runner

#---------------------------------------
# Download latest released and extract
#---------------------------------------
echo
echo "Downloading latest runner ..."

runner_file=$(get_runner_file_name ${RUNNER_VERSION} ${runner_plat})

if [ -f "${RUNNER_BASE_DIRECTORY}/runner/config.sh" ]; then
    # already extracted
    echo "${RUNNER_BASE_DIRECTORY}/runner/config.sh exists. skipping download and extract."
elif [ -f "/usr/local/etc/runner-${RUNNER_VERSION}/config.sh" ]; then
    echo "runner-${RUNNER_VERSION} cache is found. skipping download and extract."
    rm -r ./runner
    mv /usr/local/etc/runner-${RUNNER_VERSION} ./runner
elif [ -f "${runner_file}" ]; then
    echo "${runner_file} exists. skipping download."
    extract_runner ${runner_file} ${RUNNER_USER}
elif [ -f "/usr/local/etc/${runner_file}" ]; then
    echo "${runner_file} cache is found. skipping download."
    mv /usr/local/etc/${runner_file} ./
    extract_runner ${runner_file} ${RUNNER_USER}
else
    download_runner ${RUNNER_VERSION} ${runner_file}
    extract_runner ${runner_file} ${RUNNER_USER}
fi

cd ${RUNNER_BASE_DIRECTORY}/runner

#---------------------------------------
# Unattend config
#---------------------------------------
runner_url="https://github.com/${runner_scope}"
if [ -n "${ghe_hostname}" ]; then
    runner_url="${ghe_hostname}/${runner_scope}"
fi

echo
echo "Configuring ${runner_name} @ $runner_url"
{{ if eq .RunnerArg "--once" -}}
echo "./config.sh --unattended --url $runner_url --token *** --name $runner_name --labels myshoes"
${sudo_prefix}bash -c "source /etc/environment; ./config.sh --unattended --url $runner_url --token $RUNNER_TOKEN --name $runner_name --labels myshoes{{.AdditionalLabels}}"
{{ else -}}
echo "./config.sh --unattended --url $runner_url --token *** --name $runner_name --labels myshoes {{.RunnerArg}}"
${sudo_prefix}bash -c "source /etc/environment; ./config.sh --unattended --url $runner_url --token $RUNNER_TOKEN --name $runner_name --labels myshoes{{.AdditionalLabels}} {{.RunnerArg}}"
{{ end }}


#---------------------------------------
# patch once commands
#---------------------------------------
echo "apply patch file"
cat << EOF > ./bin/runsvc.sh
#!/bin/bash

# convert SIGTERM signal to SIGINT
# for more info on how to propagate SIGTERM to a child process see: http://veithen.github.io/2014/11/16/sigterm-propagation.html
trap 'kill -INT \$PID' TERM INT

if [ -f ".path" ]; then
    # configure
    export PATH=\$(cat .path)
    echo ".path=\${PATH}"
fi

# insert anything to setup env when running as a service

# run the host process which keep the listener alive
NODE_PATH="./externals/node20/bin/node"
if [ ! -e "\${NODE_PATH}" ]; then
  NODE_PATH="./externals/node16/bin/node"
fi
\${NODE_PATH} ./bin/RunnerService.js \$* &
PID=\$!
wait \$PID
trap - TERM INT
wait \$PID
EOF

cat << 'EOF' > ./bin/RunnerService.js
{{.RunnerServiceJS}}
EOF

#---------------------------------------
# Configure run commands
#---------------------------------------

# Configure job management hooks if script files exist
if [ -e "/myshoes-actions-runner-hook-job-started.sh" ]; then
	export ACTIONS_RUNNER_HOOK_JOB_STARTED="/myshoes-actions-runner-hook-job-started.sh"
fi
if [ -e "/myshoes-actions-runner-hook-job-completed.sh" ]; then
	export ACTIONS_RUNNER_HOOK_JOB_COMPLETED="/myshoes-actions-runner-hook-job-completed.sh"
fi

#---------------------------------------
# run!
#---------------------------------------

# GitHub-hosted runner load /etc/environment in /opt/runner/provisioner/provisioner.
# So, we need to load /etc/environment for job on self-hosted runner.

{{ if eq .RunnerArg "--once" -}}
echo 'bash -c "source /etc/environment; ./bin/runsvc.sh  {{.RunnerArg}}"'
${sudo_prefix}bash -c "source /etc/environment; ./bin/runsvc.sh  {{.RunnerArg}}"
{{ else -}}
echo 'bash -c "source /etc/environment; ./bin/runsvc.sh"'
${sudo_prefix}bash -c "source /etc/environment; ./bin/runsvc.sh"
{{ end }}`
