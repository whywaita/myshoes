package gh

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v35/github"
	"github.com/m4ns0ur/httpcache"
	"golang.org/x/oauth2"
)

// NewClient create a client of GitHub
func NewClient(token, gheDomain string) (*github.Client, error) {
	oauth2Transport := &oauth2.Transport{
		Source: oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		),
	}
	transport := &httpcache.Transport{
		Transport:           oauth2Transport,
		Cache:               httpCache,
		MarkCachedResponses: true,
	}

	if gheDomain == "" {
		return github.NewClient(&http.Client{Transport: transport}), nil
	}

	return github.NewEnterpriseClient(gheDomain, gheDomain, &http.Client{Transport: transport})
}

// NewClientGitHubApps create a client of GitHub using Private Key from GitHub Apps
// header is "Authorization: Bearer YOUR_JWT"
// docs: https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-a-github-app
func NewClientGitHubApps(gheDomain string) (*github.Client, error) {
	if gheDomain == "" {
		return github.NewClient(&http.Client{Transport: &appTransport}), nil
	}

	apiEndpoint, err := getAPIEndpoint(gheDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub API Endpoint: %w", err)
	}

	itr := appTransport
	itr.BaseURL = apiEndpoint.String()
	return github.NewEnterpriseClient(gheDomain, gheDomain, &http.Client{Transport: &appTransport})
}

// NewClientInstallation create a client of GitHub using installation ID from GitHub Apps
// header is "Authorization: token YOUR_INSTALLATION_ACCESS_TOKEN"
// docs: https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-an-installation
func NewClientInstallation(gheDomain string, installationID int64) (*github.Client, error) {
	itr := getInstallationTransport(installationID)

	if strings.EqualFold(gheDomain, "") || strings.EqualFold(gheDomain, "https://github.com") {
		return github.NewClient(&http.Client{Transport: itr}), nil
	}
	apiEndpoint, err := getAPIEndpoint(gheDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub API Endpoint: %w", err)
	}
	itr.BaseURL = apiEndpoint.String()
	return github.NewEnterpriseClient(gheDomain, gheDomain, &http.Client{Transport: itr})
}

func setInstallationTransport(installationID int64, itr ghinstallation.Transport) {
	installationTransports.Store(installationID, itr)
}

func getInstallationTransport(installationID int64) *ghinstallation.Transport {
	got, found := installationTransports.Load(installationID)
	if !found {
		return generateInstallationTransport(installationID)
	}

	itr, ok := got.(ghinstallation.Transport)
	if !ok {
		return generateInstallationTransport(installationID)
	}
	return &itr
}

func generateInstallationTransport(installationID int64) *ghinstallation.Transport {
	itr := ghinstallation.NewFromAppsTransport(&appTransport, installationID)
	setInstallationTransport(installationID, *itr)
	return itr
}
