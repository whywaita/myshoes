package starter

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	_ "embed" // TODO:
	"encoding/base64"
	"fmt"
	"text/template"

	"github.com/hashicorp/go-version"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
)

//go:embed scripts/RunnerService.js
var runnerService string

func getPatchedFiles() (string, error) {
	return runnerService, nil
}

func (s *Starter) getSetupScript(ctx context.Context, target datastore.Target) (string, error) {
	rawScript, err := s.getSetupRawScript(ctx, target)
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

func (s *Starter) getSetupRawScript(ctx context.Context, target datastore.Target) (string, error) {
	runnerUser := ""
	if target.RunnerUser.Valid {
		runnerUser = target.RunnerUser.String
	}
	runnerVersion, runnerTemporaryMode, err := getRunnerVersion(target.RunnerVersion)
	if err != nil {
		return "", fmt.Errorf("failed to get runner version: %w", err)
	}

	runnerServiceJs, err := getPatchedFiles()
	if err != nil {
		return "", fmt.Errorf("failed to get patched files: %w", err)
	}

	installationID, err := gh.IsInstalledGitHubApp(ctx, target.GHEDomain.String, target.Scope)
	if err != nil {
		return "", fmt.Errorf("failed to get installlation id: %w", err)
	}
	token, _, err := gh.GenerateRunnerRegistrationToken(ctx, target.GHEDomain.String, installationID, target.Scope)
	if err != nil {
		return "", fmt.Errorf("failed to generate runner register token: %w", err)
	}

	v := templateCreateLatestRunnerOnceValue{
		Scope:                   target.Scope,
		GHEDomain:               target.GHEDomain.String,
		RunnerRegistrationToken: token,
		RunnerUser:              runnerUser,
		RunnerVersion:           runnerVersion,
		RunnerServiceJS:         runnerServiceJs,
		RunnerArg:               runnerTemporaryMode.StringFlag(),
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

func getRunnerVersion(runnerVersion sql.NullString) (string, datastore.RunnerTemporaryMode, error) {
	if !runnerVersion.Valid {
		// not set, return default
		return DefaultRunnerVersion, datastore.RunnerTemporaryOnce, nil
	}

	ephemeralSupportVersion, err := version.NewVersion("v2.282.0")
	if err != nil {
		return "", datastore.RunnerTemporaryUnknown, fmt.Errorf("failed to parse ephemeral runner version: %w", err)
	}

	inputVersion, err := version.NewVersion(runnerVersion.String)
	if err != nil {
		return "", datastore.RunnerTemporaryUnknown, fmt.Errorf("failed to parse input runner version: %w", err)
	}

	if ephemeralSupportVersion.GreaterThan(inputVersion) {
		return runnerVersion.String, datastore.RunnerTemporaryOnce, nil
	}
	return runnerVersion.String, datastore.RunnerTemporaryEphemeral, nil
}

const templateCompressedScript = `#!/bin/bash

set -e

# main script compressed base64 and gzip
COMPRESSED_SCRIPT=%s
MAIN_SCRIPT_PATH=/tmp/main.sh

echo ${COMPRESSED_SCRIPT} | base64 -d | gzip -d > ${MAIN_SCRIPT_PATH}

chmod +x ${MAIN_SCRIPT_PATH}
bash -c ${MAIN_SCRIPT_PATH}`

type templateCreateLatestRunnerOnceValue struct {
	Scope                   string
	GHEDomain               string
	RunnerRegistrationToken string
	RunnerUser              string
	RunnerVersion           string
	RunnerServiceJS         string
	RunnerArg               string
}

// templateCreateLatestRunnerOnce is script template of setup runner.
// need to set runnerUser if execute using root permission. (for example, use cloud-init)
// original script: https://github.com/actions/runner/blob/80bf68db812beb298b7534012b261e6f222e004a/scripts/create-latest-svc.sh
const templateCreateLatestRunnerOnce = `#!/bin/bash

set -e

runner_scope={{.Scope}}
ghe_hostname={{.GHEDomain}}
runner_name=${3:-$(hostname)}
RUNNER_TOKEN={{.RunnerRegistrationToken}}
RUNNER_USER={{.RunnerUser}}
RUNNER_VERSION={{.RunnerVersion}}
RUNNER_BASE_DIRECTORY=/tmp  # /tmp is path of all user writable.

sudo_prefix=""
if [ $(id -u) -eq 0 ]; then  # if root
sudo_prefix="sudo -E -u ${RUNNER_USER} "
fi

export HOME="/home/${RUNNER_USER}"
export AGENT_TOOLSDIRECTORY="/opt/hostedtoolcache"

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

function install_jq()
{
    echo "jq is not installed, will be install jq."
    if [ -e /etc/debian_version ] || [ -e /etc/debian_release ]; then
        sudo apt-get update -y -qq
        sudo apt-get install -y jq
    elif [ -e /etc/redhat-release ]; then
        sudo yum install -y jq
    fi
}

function install_docker()
{
    echo "docker is not installed, will be install docker."
    if [ -e /etc/debian_version ] || [ -e /etc/debian_release ]; then
        sudo apt-get update -y -qq
        sudo apt-get install -y docker.io
    fi
}

function download_runner()
{
    runner_version=$1
    runner_plat=$2

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
which docker || fatal "docker required.  Please install in PATH with apt-get, brew, etc"


cd ${RUNNER_BASE_DIRECTORY}
${sudo_prefix}mkdir -p runner

#---------------------------------------
# Download latest released and extract
#---------------------------------------
echo
echo "Downloading latest runner ..."

version=$(echo ${RUNNER_VERSION:1})
runner_file="actions-runner-${runner_plat}-x64-${version}.tar.gz"

if [ -f "${RUNNER_BASE_DIRECTORY}/runner/config.sh" ]; then
    # already extracted
    echo "${RUNNER_BASE_DIRECTORY}/runner/config.sh exists. skipping download and extract."
elif [ -f "${runner_file}" ]; then
    echo "${runner_file} exists. skipping download."
    extract_runner ${runner_file} ${RUNNER_USER}
elif [ -f "/usr/local/etc/${runner_file}" ]; then
    echo "${runner_file} cache is found. skipping download."
    mv /usr/local/etc/${runner_file} ./
    extract_runner ${runner_file} ${RUNNER_USER}
else
    download_runner ${RUNNER_VERSION} ${runner_plat}
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
${sudo_prefix}./config.sh --unattended --url $runner_url --token $RUNNER_TOKEN --name $runner_name --labels myshoes
{{ else -}}
echo "./config.sh --unattended --url $runner_url --token *** --name $runner_name --labels myshoes {{.RunnerArg}}"
${sudo_prefix}./config.sh --unattended --url $runner_url --token $RUNNER_TOKEN --name $runner_name --labels myshoes {{.RunnerArg}}
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
./externals/node12/bin/node ./bin/RunnerService.js \$* &
PID=\$!
wait \$PID
trap - TERM INT
wait \$PID
EOF

cat << EOF > ./bin/RunnerService.js
{{.RunnerServiceJS}}
EOF

#---------------------------------------
# run!
#---------------------------------------
{{ if eq .RunnerArg "--once" -}}
echo "./bin/runsvc.sh {{.RunnerArg}}"
${sudo_prefix}./bin/runsvc.sh {{.RunnerArg}}
{{ else -}}
echo "./bin/runsvc.sh"
${sudo_prefix}./bin/runsvc.sh
{{ end }}`
