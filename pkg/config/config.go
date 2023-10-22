package config

import (
	"crypto/rsa"
	"strings"
)

// Config is config value
var Config Conf

// Conf is type of Config
type Conf struct {
	GitHub GitHubApp

	MySQLDSN        string
	Port            int
	ShoesPluginPath string
	RunnerUser      string

	Debug           bool
	Strict          bool // check to registered runner before delete job
	ModeWebhookType ModeWebhookType

	MaxConnectionsToBackend int64
	MaxConcurrencyDeleting  int64

	GitHubURL     string
	RunnerVersion string
}

// GitHubApp is type of config value
type GitHubApp struct {
	AppID     int64
	AppSecret []byte
	PEMByte   []byte
	PEM       *rsa.PrivateKey
}

// Config Environment keys
const (
	EnvGitHubAppID               = "GITHUB_APP_ID"
	EnvGitHubAppSecret           = "GITHUB_APP_SECRET"
	EnvGitHubAppPrivateKeyBase64 = "GITHUB_PRIVATE_KEY_BASE64"
	EnvMySQLURL                  = "MYSQL_URL"
	EnvPort                      = "PORT"
	EnvShoesPluginPath           = "PLUGIN"
	EnvRunnerUser                = "RUNNER_USER"
	EnvDebug                     = "DEBUG"
	EnvStrict                    = "STRICT"
	EnvModeWebhookType           = "MODE_WEBHOOK_TYPE"
	EnvMaxConnectionsToBackend   = "MAX_CONNECTIONS_TO_BACKEND"
	EnvMaxConcurrencyDeleting    = "MAX_CONCURRENCY_DELETING"
	EnvGitHubURL                 = "GITHUB_URL"
	EnvRunnerVersion             = "RUNNER_VERSION"
)

// ModeWebhookType is type value for GitHub webhook
type ModeWebhookType int

const (
	// ModeWebhookTypeUnknown is unknown
	ModeWebhookTypeUnknown ModeWebhookType = iota
	// ModeWebhookTypeCheckRun is check_run
	ModeWebhookTypeCheckRun
	// ModeWebhookTypeWorkflowJob is workflow_job
	ModeWebhookTypeWorkflowJob
)

// String is implementation of fmt.Stringer
func (mwt ModeWebhookType) String() string {
	unknown := "unknown"
	switch mwt {
	case ModeWebhookTypeUnknown:
		return unknown
	case ModeWebhookTypeCheckRun:
		return "check_run"
	case ModeWebhookTypeWorkflowJob:
		return "workflow_job"
	}

	return unknown
}

// Equal check in and value
func (mwt ModeWebhookType) Equal(in string) bool {
	return strings.EqualFold(in, mwt.String())
}

func marshalModeWebhookType(in string) ModeWebhookType {
	switch in {
	case "check_run":
		return ModeWebhookTypeCheckRun
	case "workflow_job":
		return ModeWebhookTypeWorkflowJob
	}

	return ModeWebhookTypeUnknown
}

// IsGHES return myshoes for GitHub Enterprise Server
func (c Conf) IsGHES() bool {
	return !strings.EqualFold(c.GitHubURL, "https://github.com")
}
