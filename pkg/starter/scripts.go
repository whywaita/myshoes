//go:generate statik -src=./scripts

package starter

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"github.com/rakyll/statik/fs"

	"github.com/whywaita/myshoes/pkg/datastore"
	_ "github.com/whywaita/myshoes/pkg/starter/statik" // TODO:
)

func getPatchedFiles() (string, error) {
	statikFs, err := fs.New()
	if err != nil {
		return "", fmt.Errorf("failed to New statik filesystem: %w", err)
	}
	r, err := statikFs.Open("/RunnerService.js")
	if err != nil {
		return "", fmt.Errorf("failed to open RunnerService.js: %w", err)
	}
	defer r.Close()
	rsjs, err := ioutil.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read RunnerService.js: %w", err)
	}

	return fmt.Sprintf("%s", rsjs), nil
}

func (s *Starter) getSetupScript(target datastore.Target) (string, error) {
	rawScript, err := s.getSetupRawScript(target)
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

func (s *Starter) getSetupRawScript(target datastore.Target) (string, error) {
	runnerUser := ""
	if target.RunnerUser.Valid {
		runnerUser = target.RunnerUser.String
	}
	targetVersion := ""
	if target.RunnerVersion.Valid {
		targetVersion = target.RunnerVersion.String
	} else {
		targetVersion = DefaultRunnerVersion
	}

	runnerServiceJs, err := getPatchedFiles()
	if err != nil {
		return "", fmt.Errorf("failed to get patched files: %w", err)
	}

	script := fmt.Sprintf(templateCreateLatestRunnerOnce,
		target.Scope,
		target.GHEDomain.String,
		target.GitHubPersonalToken,
		runnerUser,
		targetVersion,
		runnerServiceJs)

	return script, nil
}

const templateCompressedScript = `#!/bin/bash

set -e

# main script compressed base64 and gzip
COMPRESSED_SCRIPT=%s
MAIN_SCRIPT_PATH=/tmp/main.sh

echo ${COMPRESSED_SCRIPT} | base64 -d | gzip -d > ${MAIN_SCRIPT_PATH}

chmod +x ${MAIN_SCRIPT_PATH}
bash -c ${MAIN_SCRIPT_PATH}`

// templateCreateLatestRunnerOnce is script template of setup runner.
// need to set runnerUser if execute using root permission. (for example, use cloud-init)
// original script: https://github.com/actions/runner/blob/80bf68db812beb298b7534012b261e6f222e004a/scripts/create-latest-svc.sh
const templateCreateLatestRunnerOnce = `#!/bin/bash

set -e

#
# Downloads latest releases (not pre-release) runner
# Configures as a service
#
# Examples:
# RUNNER_CFG_PAT=<yourPAT> ./create-latest-svc.sh myuser/myrepo my.ghe.deployment.net
# RUNNER_CFG_PAT=<yourPAT> ./create-latest-svc.sh myorg my.ghe.deployment.net
#
# Usage:
#     export RUNNER_CFG_PAT=<yourPAT>
#     ./create-latest-svc scope [ghe_domain] [name] [user]
#
#      scope       required  repo (:owner/:repo) or org (:organization)
#      ghe_domain  optional  the fully qualified domain name of your GitHub Enterprise Server deployment
#      name        optional  defaults to hostname
#      user        optional  user svc will run as. defaults to current
#
# Notes:
# PATS over envvars are more secure
# Should be used on VMs and not containers
# Works on OSX and Linux
# Assumes x64 arch
#

runner_scope=%s
ghe_hostname=%s
runner_name=${3:-$(hostname)}
svc_user=${4:-$USER}
RUNNER_CFG_PAT=%s
RUNNER_USER=%s
RUNNER_VERSION=%s

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

if [ -z "${runner_scope}" ]; then fatal "supply scope as argument 1"; fi
if [ -z "${RUNNER_CFG_PAT}" ]; then fatal "RUNNER_CFG_PAT must be set before calling"; fi

which curl || fatal "curl required.  Please install in PATH with apt-get, brew, etc"
which jq || install_jq
which jq || fatal "jq required.  Please install in PATH with apt-get, brew, etc"
which docker || install_docker
which docker || fatal "docker required.  Please install in PATH with apt-get, brew, etc"

# move /tmp, /tmp is path of all user writable.
cd /tmp

# bail early if there's already a runner there. also sudo early
if [ -d ./runner ]; then
    fatal "Runner already exists.  Use a different directory or delete ./runner"
fi

${sudo_prefix}mkdir -p runner

# TODO: validate not in a container
# TODO: validate systemd or osx svc installer

#--------------------------------------
# Get a config token
#--------------------------------------
echo
echo "Generating a registration token..."

base_api_url="https://api.github.com"
if [ -n "${ghe_hostname}" ]; then
    base_api_url="${ghe_hostname}/api/v3"
fi

# if the scope has a slash, it's a repo runner
orgs_or_repos="orgs"
if [[ "$runner_scope" == *\/* ]]; then
    orgs_or_repos="repos"
fi

export RUNNER_TOKEN=$(curl -s -X POST ${base_api_url}/${orgs_or_repos}/${runner_scope}/actions/runners/registration-token -H "accept: application/vnd.github.everest-preview+json" -H "authorization: token ${RUNNER_CFG_PAT}" | jq -r '.token')

if [ "null" == "$RUNNER_TOKEN" -o -z "$RUNNER_TOKEN" ]; then fatal "Failed to get a token"; fi

#---------------------------------------
# Download latest released and extract
#---------------------------------------
echo
echo "Downloading latest runner ..."

# For the GHES Alpha, download the runner from github.com
#latest_version_label=$(curl -s -X GET 'https://api.github.com/repos/actions/runner/releases/latest' | jq -r '.tag_name')
#latest_version=$(echo ${latest_version_label:1})
version=$(echo ${RUNNER_VERSION:1})
runner_file="actions-runner-${runner_plat}-x64-${version}.tar.gz"

if [ -f "${runner_file}" ]; then
    echo "${runner_file} exists. skipping download."
elif [ -f "/usr/local/etc/${runner_file}" ]; then
    echo "${runner_file} cache is found. skipping download."
    mv /usr/local/etc/${runner_file} ./
else
    runner_url="https://github.com/actions/runner/releases/download/${RUNNER_VERSION}/${runner_file}"

    echo "Downloading ${version_label} for ${runner_plat} ..."
    echo $runner_url

    curl -O -L ${runner_url}
fi

ls -la *.tar.gz

#---------------------------------------------------
# extract to runner directory in this directory
#---------------------------------------------------
echo
echo "Extracting ${runner_file} to ./runner"

tar xzf "./${runner_file}" -C runner

# export of pass
if [ $(id -u) -eq 0 ]; then
chown -R ${RUNNER_USER} ./runner
fi

pushd ./runner

#---------------------------------------
# Unattend config
#---------------------------------------
runner_url="https://github.com/${runner_scope}"
if [ -n "${ghe_hostname}" ]; then
    runner_url="${ghe_hostname}/${runner_scope}"
fi

echo
echo "Configuring ${runner_name} @ $runner_url"
echo "./config.sh --unattended --url $runner_url --token *** --name $runner_name --labels myshoes"
${sudo_prefix}./config.sh --unattended --url $runner_url --token $RUNNER_TOKEN --name $runner_name --labels myshoes

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
%s
EOF

#---------------------------------------
# run!
#---------------------------------------
echo "./bin/runsvc.sh --once"
${sudo_prefix}./bin/runsvc.sh --once`
