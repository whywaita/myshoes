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
)

// Load load config from environment
func Load() {
	appID, err := strconv.ParseInt(os.Getenv(EnvGitHubAppID), 10, 64)
	if err != nil {
		log.Panicf("failed to parse %s: %+v", EnvGitHubAppID, err)
	}
	Config.GitHub.AppID = appID

	pemBase64ed := os.Getenv(EnvGitHubAppPrivateKeyBase64)
	if pemBase64ed == "" {
		log.Panicf("%s must be set", EnvGitHubAppPrivateKeyBase64)
	}
	pemByte, err := base64.StdEncoding.DecodeString(pemBase64ed)
	if err != nil {
		log.Panicf("failed to decode base64 %s: %+v", EnvGitHubAppPrivateKeyBase64, err)
	}
	Config.GitHub.PEMByte = pemByte
	block, _ := pem.Decode(pemByte)
	if block == nil {
		log.Panicf("%s is invalid format, please input private key ", EnvGitHubAppPrivateKeyBase64)
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Panicf("%s is invalid format, failed to parse private key: %+v", EnvGitHubAppPrivateKeyBase64, err)
	}
	Config.GitHub.PEM = key

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
	Config.ShoesPluginPath = absPath
	log.Printf("use plugin path is %s\n", Config.ShoesPluginPath)

	debug := os.Getenv(EnvDebug)
	if debug == "true" {
		Config.Debug = true
	} else {
		Config.Debug = false
	}

	Config.Strict = true
	if os.Getenv(EnvStrict) == "false" {
		Config.Strict = false
	}

	Config.ModeWebhookType = ModeWebhookTypeCheckRun
	if os.Getenv(EnvModeWebhookType) != "" {
		mwt := marshalModeWebhookType(os.Getenv(EnvModeWebhookType))

		if mwt == ModeWebhookTypeUnknown {
			log.Panicf("%s is invalid webhook type", os.Getenv(EnvModeWebhookType))
		}
		Config.ModeWebhookType = mwt
	}

	Config.MaxConnectionsToBackend = 50
	if os.Getenv(EnvMaxConnectionsToBackend) != "" {
		numberPB, err := strconv.ParseInt(os.Getenv(EnvMaxConnectionsToBackend), 10, 64)
		if err != nil {
			log.Panicf("failed to convert int64 %s: %+v", EnvMaxConnectionsToBackend, err)
		}
		Config.MaxConnectionsToBackend = numberPB
	}
	Config.MaxConcurrencyDeleting = 1
	if os.Getenv(EnvMaxConcurrencyDeleting) != "" {
		numberCD, err := strconv.ParseInt(os.Getenv(EnvMaxConcurrencyDeleting), 10, 64)
		if err != nil {
			log.Panicf("failed to convert int64 %s: %+v", EnvMaxConcurrencyDeleting, err)
		}
		Config.MaxConcurrencyDeleting = numberCD
	}
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
		return "", fmt.Errorf("unsupported fetch schema")
	}
}

// fetchHTTP fetch plugin binary over HTTP(s).
// save to current directory.
func fetchHTTP(u *url.URL) (string, error) {
	log.Printf("fetch plugin binary from %s\n", u.String())
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to working directory: %w", err)
	}

	p := strings.Split(u.Path, "/")
	fileName := p[len(p)-1]

	fp := filepath.Join(pwd, fileName)
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
