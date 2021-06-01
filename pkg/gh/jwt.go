package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/whywaita/myshoes/internal/config"
)

// function pointers (for testing)
var (
	GHlistInstallations     = listInstallations
	GHlistAppsInstalledRepo = listAppsInstalledRepo
)

// GenerateGitHubAppsToken generate token of GitHub Apps using private key
func GenerateGitHubAppsToken(gheDomain string, installationID int64) (string, *time.Time, error) {
	p := path.Join(
		"app",
		"installations",
		strconv.FormatInt(installationID, 10),
		"access_tokens")
	jb, err := callAPIPrivateKey(http.MethodPost, p, gheDomain)
	if err != nil {
		return "", nil, fmt.Errorf("failed to call API: %w", err)
	}

	it := new(github.InstallationToken)
	if err := json.Unmarshal(jb, it); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return *it.Token, it.ExpiresAt, nil
}

// IsInstalledGitHubApp check installed GitHub Apps in gheDomain + inputScope
func IsInstalledGitHubApp(ctx context.Context, gheDomain, inputScope string) (int64, error) {
	installations, err := GHlistInstallations(gheDomain)
	if err != nil {
		return -1, fmt.Errorf("failed to get list of installations: %w", err)
	}

	for _, i := range installations {
		if i.SuspendedAt != nil {
			continue
		}

		if strings.HasPrefix(inputScope, *i.Account.Login) {
			// i.Account.Login is username or Organization name.
			// e.g.) `https://github.com/example/sample` -> `example/sample`
			// strings.HasPrefix search scope include i.Account.Login.

			switch {
			case strings.EqualFold(*i.RepositorySelection, "all"):
				// "all" can use GitHub Apps in all repositories that joined i.Account.Login.
				return *i.ID, nil
			case strings.EqualFold(*i.RepositorySelection, "selected"):
				// "selected" can use GitHub Apps in only some repositories that permitted.
				// So, need to check more using other endpoint.
				err := isInstalledGitHubAppSelected(ctx, gheDomain, inputScope, *i.ID)
				if err == nil {
					// found
					return *i.ID, nil
				}
			}
		}
	}

	return -1, fmt.Errorf("%s/%s is not installed configured GitHub Apps", gheDomain, inputScope)
}

func isInstalledGitHubAppSelected(ctx context.Context, gheDomain, inputScope string, installationID int64) error {
	lr, err := GHlistAppsInstalledRepo(ctx, gheDomain, installationID)
	if err != nil {
		return fmt.Errorf("failed to get list of installed repositories: %w", err)
	}

	s := DetectScope(inputScope)
	switch {
	case *lr.TotalCount <= 0:
		return fmt.Errorf("installed repository is not found")
	case s == Organization:
		// Scope is Organization and installed repository is exist
		// So GitHub Apps installed
		return nil
	case s != Repository:
		return fmt.Errorf("scope is unknown: %s", s)
	}

	// s == Repository
	for _, repo := range lr.Repositories {
		if strings.EqualFold(*repo.FullName, inputScope) {
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func listAppsInstalledRepo(ctx context.Context, gheDomain string, installationID int64) (*github.ListRepositories, error) {
	token, _, err := GenerateGitHubAppsToken(gheDomain, installationID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate GitHub Apps Token: %w", err)
	}
	client, err := NewClient(ctx, token, gheDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to NewClient: %w", err)
	}

	lr, _, err := client.Apps.ListRepos(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get installed repositories: %w", err)
	}

	return lr, nil
}

func listInstallations(gheDomain string) ([]*github.Installation, error) {
	p := path.Join(
		"app",
		"installations")
	jb, err := callAPIPrivateKey(http.MethodGet, p, gheDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}

	is := new([]*github.Installation)
	if err := json.Unmarshal(jb, is); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return *is, nil
}

func callAPIPrivateKey(method, apiPath, gheDomain string) ([]byte, error) {
	jwtToken, err := generateJWT(time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT token: %w", err)
	}
	apiEndpoint, err := getAPIEndpoint(gheDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get API endpoint: %w", err)
	}

	p := path.Join(apiEndpoint.Path, apiPath)
	apiEndpoint.Path = p
	req, err := http.NewRequest(method, apiEndpoint.String(), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", jwtToken))
	req.Header.Add("Accept", "application/vnd.github.v3+json")

	client := http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to POST request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode > 400 {
		return nil, fmt.Errorf("invalid status code (%d)", resp.StatusCode)
	}

	jb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return jb, nil
}

func generateJWT(baseTime time.Time) ([]byte, error) {
	privateKey := config.Config.GitHub.PEM
	appID := strconv.Itoa(int(config.Config.GitHub.AppID))

	if privateKey == nil {
		return nil, fmt.Errorf("GitHub Apps private key is not configured")
	}

	token := jwt.New()
	token.Set(jwt.IssuedAtKey, baseTime.Add(-1*time.Minute))
	token.Set(jwt.ExpirationKey, baseTime.Add(10*time.Minute))
	token.Set(jwt.IssuerKey, appID)

	signed, err := jwt.Sign(token, jwa.RS256, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign JWT: %w", err)
	}

	return signed, nil
}
