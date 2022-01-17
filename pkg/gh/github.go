package gh

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/gregjones/httpcache"
)

var (
	// ErrNotFound is error for not found
	ErrNotFound = fmt.Errorf("not found")

	// rateLimitRemain is remaining of Rate limit, for metrics
	rateLimitRemain = sync.Map{}
	// rateLimitLimit is limit of Rate limit, for metrics
	rateLimitLimit = sync.Map{}

	// httpCache is shareable response cache
	httpCache = httpcache.NewMemoryCache()
	// appTransport is transport for GitHub Apps
	appTransport = ghinstallation.AppsTransport{}
	// installationTransports is map of ghinstallation.Transport for cache token of installation.
	// key: installationID, value: ghinstallation.Transport
	installationTransports = sync.Map{}
)

// InitializeCache create a cache
func InitializeCache(appID int64, appPEM []byte) error {
	tr := httpcache.NewTransport(httpCache)
	itr, err := ghinstallation.NewAppsTransport(tr, appID, appPEM)
	if err != nil {
		return fmt.Errorf("failed to create Apps transport: %w", err)
	}
	appTransport = *itr
	return nil
}

// CheckSignature check trust installation id from event.
func CheckSignature(installationID int64) error {
	if itr := ghinstallation.NewFromAppsTransport(&appTransport, installationID); itr == nil {
		return fmt.Errorf("failed to create GitHub installation")
	}

	return nil
}

// ExistGitHubRepository check exist of GitHub repository
func ExistGitHubRepository(scope, gheDomain string, accessToken string) error {
	repoURL, err := getRepositoryURL(scope, gheDomain)
	if err != nil {
		return fmt.Errorf("failed to get repository url: %w", err)
	}

	client := http.DefaultClient
	req, err := http.NewRequest(http.MethodGet, repoURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("token %s", accessToken))

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

func getRepositoryURL(scope, gheDomain string) (string, error) {
	// github.com
	//   => https://api.github.com/repos/:owner/:repo
	//   => https://api.github.com/orgs/:owner
	// GitHub Enterprise Server
	//   => https://{your_ghe_server_url}/api/repos/:owner/:repo
	//   => https://{your_ghe_server_url}/api/orgs/:owner

	s := DetectScope(scope)
	if s == Unknown {
		return "", fmt.Errorf("failed to detect valid scope")
	}

	apiEndpoint, err := getAPIEndpoint(gheDomain)
	if err != nil {
		return "", fmt.Errorf("failed to get API Endpoint: %w", err)
	}

	p := path.Join(apiEndpoint.Path, s.String(), scope)
	apiEndpoint.Path = p

	return apiEndpoint.String(), nil
}

func getAPIEndpoint(gheDomain string) (*url.URL, error) {
	var apiEndpoint *url.URL
	if gheDomain != "" {
		u, err := url.Parse(gheDomain)
		if err != nil {
			return nil, fmt.Errorf("failed to parse GHE url: %w", err)
		}

		p := u.Path
		p = path.Join(p, "api", "v3")
		u.Path = p
		apiEndpoint = u
	} else {
		u, err := url.Parse("https://api.github.com")
		if err != nil {
			return nil, fmt.Errorf("failed to parse github.com: %w", err)
		}
		apiEndpoint = u
	}

	return apiEndpoint, nil
}
