package config

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-version"
)

// Load load config from environment
func Load() {
	c := LoadWithDefault()

	ga := LoadGitHubApps()
	c.GitHub = *ga

	pluginPath := LoadPluginPath()
	c.ShoesPluginPath = pluginPath

	Config = c
}

// LoadWithDefault load only value that has default value
func LoadWithDefault() Conf {
	var c Conf

	p := "8080"
	if os.Getenv(EnvPort) != "" {
		p = os.Getenv(EnvPort)
	}
	pp, err := strconv.Atoi(p)
	if err != nil {
		log.Panicf("failed to parse PORT: %+v", err)
	}
	c.Port = pp

	runnerUser := "runner"
	if os.Getenv(EnvRunnerUser) != "" {
		runnerUser = os.Getenv(EnvRunnerUser)
	}
	c.RunnerUser = runnerUser

	c.Debug = false
	if os.Getenv(EnvDebug) == "true" {
		c.Debug = true
	}

	c.Strict = true
	if os.Getenv(EnvStrict) == "false" {
		c.Strict = false
	}

	c.ModeWebhookType = ModeWebhookTypeCheckRun
	if os.Getenv(EnvModeWebhookType) != "" {
		mwt := marshalModeWebhookType(os.Getenv(EnvModeWebhookType))

		if mwt == ModeWebhookTypeUnknown {
			log.Panicf("%s is invalid webhook type", os.Getenv(EnvModeWebhookType))
		}
		c.ModeWebhookType = mwt
	}

	c.MaxConnectionsToBackend = 50
	if os.Getenv(EnvMaxConnectionsToBackend) != "" {
		numberPB, err := strconv.ParseInt(os.Getenv(EnvMaxConnectionsToBackend), 10, 64)
		if err != nil {
			log.Panicf("failed to convert int64 %s: %+v", EnvMaxConnectionsToBackend, err)
		}
		c.MaxConnectionsToBackend = numberPB
	}
	c.MaxConcurrencyDeleting = 1
	if os.Getenv(EnvMaxConcurrencyDeleting) != "" {
		numberCD, err := strconv.ParseInt(os.Getenv(EnvMaxConcurrencyDeleting), 10, 64)
		if err != nil {
			log.Panicf("failed to convert int64 %s: %+v", EnvMaxConcurrencyDeleting, err)
		}
		c.MaxConcurrencyDeleting = numberCD
	}

	c.GitHubURL = "https://github.com"
	if os.Getenv(EnvGitHubURL) != "" {
		u, err := url.Parse(os.Getenv(EnvGitHubURL))
		if err != nil {
			log.Panicf("failed to parse URL %s: %+v", os.Getenv(EnvGitHubURL), err)
		}

		if strings.EqualFold(u.Scheme, "") {
			log.Panicf("%s must has scheme (value: %s)", EnvGitHubURL, os.Getenv(EnvGitHubURL))
		}
		if strings.EqualFold(u.Host, "") {
			log.Panicf("%s must has host (value: %s)", EnvGitHubURL, os.Getenv(EnvGitHubURL))
		}

		c.GitHubURL = os.Getenv(EnvGitHubURL)
	}

	if os.Getenv(EnvRunnerVersion) == "" {
		c.RunnerVersion = "latest"
	} else {
		// valid value: "latest" or "vX.XXX.X"
		switch os.Getenv(EnvRunnerVersion) {
		case "latest":
			c.RunnerVersion = "latest"
		default:
			_, err := version.NewVersion(os.Getenv(EnvRunnerVersion))
			if err != nil {
				log.Panicf("failed to parse input runner version: %+v", err)
			}

			c.RunnerVersion = os.Getenv(EnvRunnerVersion)
		}
	}

	c.ShoesPluginOutputPath = "."
	if os.Getenv(EnvShoesPluginOutputPath) != "" {
		c.ShoesPluginOutputPath = os.Getenv(EnvShoesPluginOutputPath)
	}

	Config = c
	return c
}

// LoadGitHubApps load config for GitHub Apps
func LoadGitHubApps() *GitHubApp {
	var ga GitHubApp
	appID, err := strconv.ParseInt(os.Getenv(EnvGitHubAppID), 10, 64)
	if err != nil {
		log.Panicf("failed to parse %s: %+v", EnvGitHubAppID, err)
	}
	ga.AppID = appID

	pemBase64ed := os.Getenv(EnvGitHubAppPrivateKeyBase64)
	if pemBase64ed == "" {
		log.Panicf("%s must be set", EnvGitHubAppPrivateKeyBase64)
	}
	pemByte, err := base64.StdEncoding.DecodeString(pemBase64ed)
	if err != nil {
		log.Panicf("failed to decode base64 %s: %+v", EnvGitHubAppPrivateKeyBase64, err)
	}
	ga.PEMByte = pemByte

	block, _ := pem.Decode(pemByte)
	if block == nil {
		log.Panicf("%s is invalid format, please input private key ", EnvGitHubAppPrivateKeyBase64)
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Panicf("%s is invalid format, failed to parse private key: %+v", EnvGitHubAppPrivateKeyBase64, err)
	}
	ga.PEM = key

	appSecret := os.Getenv(EnvGitHubAppSecret)
	if appSecret == "" {
		log.Panicf("%s must be set", EnvGitHubAppSecret)
	}
	ga.AppSecret = []byte(appSecret)

	return &ga
}

// LoadMySQLURL load MySQL URL from environment
func LoadMySQLURL() string {
	mysqlURL := os.Getenv(EnvMySQLURL)
	if mysqlURL == "" {
		log.Panicf("%s must be set", EnvMySQLURL)
	}
	return mysqlURL
}

// LoadPluginPath load plugin path from environment
func LoadPluginPath() string {
	pluginPath := os.Getenv(EnvShoesPluginPath)
	if pluginPath == "" {
		log.Panicf("%s must be set", EnvShoesPluginPath)
	}
	fp, err := fetch(pluginPath)
	if err != nil {
		log.Panicf("failed to fetch plugin binary: %+v", err)
	}
	absPath, err := checkBinary(fp)
	if err != nil {
		log.Panicf("failed to check plugin binary: %+v", err)
	}
	log.Printf("use plugin path is %s\n", absPath)
	return absPath
}

func checkBinary(p string) (string, error) {
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	// need permission of execute
	if err := os.Chmod(p, 0777); err != nil {
		return "", fmt.Errorf("failed to chmod: %w", err)
	}

	if filepath.IsAbs(p) {
		return p, nil
	}

	apath, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("failed to get abs: %w", err)
	}

	return apath, nil
}

// fetch retrieve plugin binaries.
// return saved file path.
func fetch(p string) (string, error) {
	_, err := os.Stat(p)
	if err == nil {
		// this is file path!
		return p, nil
	}

	u, err := url.Parse(p)
	if err != nil {
		return "", fmt.Errorf("failed to parse input url: %w", err)
	}
	switch u.Scheme {
	case "http", "https":
		return fetchHTTP(u)
	default:
		return "", fmt.Errorf("unsupported fetch schema (scheme: %s)", u.Scheme)
	}
}

// fetchHTTP fetch plugin binary over HTTP(s).
// save to current directory.
func fetchHTTP(u *url.URL) (string, error) {
	log.Printf("fetch plugin binary from %s\n", u.String())
	dir := Config.ShoesPluginOutputPath
	if strings.EqualFold(dir, ".") {
		pwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to working directory: %w", err)
		}
		dir = pwd
	}

	p := strings.Split(u.Path, "/")
	fileName := p[len(p)-1]

	fp := filepath.Join(dir, fileName)
	f, err := os.Create(fp)
	if err != nil {
		return "", fmt.Errorf("failed to create os file: %w", err)
	}
	defer f.Close()

	resp, err := http.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("failed to get config via HTTP(S): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		_, err := io.Copy(f, resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to write file (path: %s): %w", fp, err)
		}
	}

	return fp, nil
}
