package gh

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/whywaita/myshoes/pkg/datastore"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v32/github"
	"github.com/whywaita/myshoes/internal/config"
)

// NewClient create a client of GitHub
func NewClient(installationID int64) (*github.Client, error) {
	appID := config.Config.GitHub.AppID
	pem := config.Config.GitHub.PEM

	tr := http.DefaultTransport
	itr, err := ghinstallation.New(tr, appID, installationID, pem)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub installation: %w", err)
	}

	return github.NewClient(&http.Client{
		Transport: itr,
		Timeout:   5 * time.Second,
	}), nil
}

// ExistGitHubRepository check exist of github repository
func ExistGitHubRepository(t datastore.Target) error {
	repoURL, err := getRepositoryURL(t.Scope, t.GHEDomain.String, t.GHEDomain.Valid)
	if err != nil {
		return fmt.Errorf("failed to get repository url: %w", err)
	}

	client := http.DefaultClient
	req, err := http.NewRequest(http.MethodGet, repoURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("token %s", t.GitHubPersonalToken))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return nil
	} else if resp.StatusCode == http.StatusNotFound {
		return errors.New("not found")
	}

	return fmt.Errorf("invalid response code (%d)", resp.StatusCode)
}

func getRepositoryURL(scope, gheDomain string, gheDomainValid bool) (string, error) {
	// github.com
	//   => https://api.github.com/repos/:owner/:repo
	//   => https://api.github.com/orgs/:owner
	// GitHub Enterprise Server
	//   => https://{your_ghe_server_url}/api/repos/:owner/:repo
	//   => https://{your_ghe_server_url}/api/orgs/:owner

	s := detectScope(scope)
	if s == Unknown {
		return "", fmt.Errorf("failed to detect valid scope")
	}

	var apiEndpoint *url.URL
	if gheDomainValid {
		u, err := url.Parse(gheDomain)
		if err != nil {
			return "", fmt.Errorf("failed to parse GHE url: %w", err)
		}

		u.Path = path.Join(u.Path, "api")
		apiEndpoint = u
	} else {
		u, err := url.Parse("https://api.github.com")
		if err != nil {
			return "", fmt.Errorf("failed to parse github.com: %w", err)
		}
		apiEndpoint = u
	}

	p := path.Join(apiEndpoint.Path, s.String(), scope)
	apiEndpoint.Path = p

	return apiEndpoint.String(), nil
}

type Scope int

const (
	Unknown Scope = iota
	Repository
	Organization
)

func (s Scope) String() string {
	switch s {
	case Repository:
		return "repos"
	case Organization:
		return "orgs"
	default:
		return "unknown"
	}
}

func detectScope(scope string) Scope {
	sep := strings.Split(scope, "/")
	switch len(sep) {
	case 1:
		return Organization
	case 2:
		return Repository
	default:
		return Unknown
	}
}
