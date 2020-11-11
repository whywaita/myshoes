package config

import (
	"encoding/base64"
	"log"
	"os"
	"strconv"
)

var Config conf

type conf struct {
	GitHub struct {
		AppID     int64
		AppSecret []byte
		PEM       []byte
	}

	MySQLDSN string
	Port     int
}

// Config Environment keys
const (
	EnvGitHubAppID               = "GITHUB_APP_ID"
	EnvGitHubAppSecret           = "GITHUB_APP_SECRET"
	EnvGitHubAppPrivateKeyBase64 = "GITHUB_PRIVATE_KEY_BASE64"
	EnvMySQLURL                  = "MYSQL_URL"
	EnvPort                      = "PORT"
)

func init() {
	appID, err := strconv.ParseInt(os.Getenv(EnvGitHubAppID), 10, 64)
	if err != nil {
		log.Panicf("failed to parse %s: %+v", EnvGitHubAppID, err)
	}
	Config.GitHub.AppID = appID

	pemBase64ed := os.Getenv(EnvGitHubAppPrivateKeyBase64)
	if pemBase64ed == "" {
		log.Panicf("%s must be set", EnvGitHubAppPrivateKeyBase64)
	}
	pem, err := base64.StdEncoding.DecodeString(pemBase64ed)
	if err != nil {
		log.Panicf("failed to decode base64 %s: %+v", EnvGitHubAppPrivateKeyBase64, err)
	}
	Config.GitHub.PEM = pem

	appSecret := os.Getenv(EnvGitHubAppSecret)
	if appSecret == "" {
		log.Panicf("%s must be set", EnvGitHubAppSecret)
	}
	Config.GitHub.AppSecret = []byte(appSecret)

	mysqlURL := os.Getenv(EnvMySQLURL)
	if mysqlURL == "" {
		log.Panicf("%s must be set", EnvMySQLURL)
	}
	Config.MySQLDSN = mysqlURL

	p := os.Getenv(EnvPort)
	if p == "" {
		p = "8080"
	}
	pp, err := strconv.Atoi(p)
	if err != nil {
		log.Panicf("failed to parse PORT: %+v", err)
	}
	Config.Port = pp
}
