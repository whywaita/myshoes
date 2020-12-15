package config

// Config is config value
var Config conf

type conf struct {
	GitHub struct {
		AppID     int64
		AppSecret []byte
		PEM       []byte
	}

	MySQLDSN        string
	Port            int
	ShoesPluginPath string

	RunnerUser string
}

// Config Environment keys
const (
	EnvGitHubAppID               = "GITHUB_APP_ID"
	EnvGitHubAppSecret           = "GITHUB_APP_SECRET"
	EnvGitHubAppPrivateKeyBase64 = "GITHUB_PRIVATE_KEY_BASE64"
	EnvMySQLURL                  = "MYSQL_URL"
	EnvPort                      = "PORT"
	EnvShoesPluginPath           = "PLUGIN"
)
