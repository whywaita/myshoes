package config

import "crypto/rsa"

// Config is config value
var Config conf

type conf struct {
	GitHub struct {
		AppID     int64
		AppSecret []byte
		PEMByte   []byte
		PEM       *rsa.PrivateKey
	}

	MySQLDSN        string
	Port            int
	ShoesPluginPath string
	RunnerUser      string

	Debug                   bool
	Strict                  bool // check to registered runner before delete job
	MaxConnectionsToBackend int64
	MaxConcurrencyDeleting  int64

	EndpointActionsRunnerRelease string
}

// Config Environment keys
const (
	EnvGitHubAppID                  = "GITHUB_APP_ID"
	EnvGitHubAppSecret              = "GITHUB_APP_SECRET"
	EnvGitHubAppPrivateKeyBase64    = "GITHUB_PRIVATE_KEY_BASE64"
	EnvMySQLURL                     = "MYSQL_URL"
	EnvPort                         = "PORT"
	EnvShoesPluginPath              = "PLUGIN"
	EnvDebug                        = "DEBUG"
	EnvStrict                       = "STRICT"
	EnvMaxConnectionsToBackend      = "MAX_CONNECTIONS_TO_BACKEND"
	EnvMaxConcurrencyDeleting       = "MAX_CONCURRENCY_DELETING"
	EnvEndpointActionsRunnerRelease = "ENDPOINT_ACTIONS_RUNNER_RELEASE"
)

const (
	// DefaultRunnerVersion is default value of actions/runner
	DefaultRunnerVersion = "v2.275.1"
)
