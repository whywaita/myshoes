package gh

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v35/github"
	"github.com/m4ns0ur/httpcache"
	"github.com/patrickmn/go-cache"
	"github.com/whywaita/myshoes/pkg/logger"
	"golang.org/x/oauth2"
)

var (
	// ErrNotFound is error for not found
	ErrNotFound = fmt.Errorf("not found")

	// ResponseCache is cache variable
	responseCache = cache.New(5*time.Minute, 10*time.Minute)

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

// ExistGitHubRunner check exist registered of GitHub runner
func ExistGitHubRunner(ctx context.Context, client *github.Client, owner, repo, runnerName string) (*github.Runner, error) {
	runners, err := ListRunners(ctx, client, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of runners: %w", err)
	}

	return ExistGitHubRunnerWithRunner(runners, runnerName)
}

// ExistGitHubRunnerWithRunner check exist registered of GitHub runner from a list of runner
func ExistGitHubRunnerWithRunner(runners []*github.Runner, runnerName string) (*github.Runner, error) {
	for _, r := range runners {
		if strings.EqualFold(r.GetName(), runnerName) {
			return r, nil
		}
	}

	return nil, ErrNotFound
}

// ListRunners get runners that registered repository or org
func ListRunners(ctx context.Context, client *github.Client, owner, repo string) ([]*github.Runner, error) {
	if cachedRs, found := responseCache.Get(getCacheKey(owner, repo)); found {
		return cachedRs.([]*github.Runner), nil
	}

	var opts = &github.ListOptions{
		Page:    0,
		PerPage: 100,
	}

	var rs []*github.Runner
	for {
		logger.Logf(true, "get runners from GitHub, page: %d, now all runners: %d", opts.Page, len(rs))
		runners, resp, err := listRunners(ctx, client, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list runners: %w", err)
		}
		storeRateLimit(getRateLimitKey(owner, repo), resp.Rate)

		rs = append(rs, runners.Runners...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	responseCache.Set(getCacheKey(owner, repo), rs, 1*time.Second)
	logger.Logf(true, "found %d runners in GitHub", len(rs))

	return rs, nil
}

func getCacheKey(owner, repo string) string {
	return fmt.Sprintf("owner-%s-repo-%s", owner, repo)
}

func listRunners(ctx context.Context, client *github.Client, owner, repo string, opts *github.ListOptions) (*github.Runners, *github.Response, error) {
	if repo == "" {
		runners, resp, err := client.Actions.ListOrganizationRunners(ctx, owner, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list organization runners: %w", err)
		}
		return runners, resp, nil
	}

	runners, resp, err := client.Actions.ListRunners(ctx, owner, repo, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list repository runners: %w", err)
	}
	return runners, resp, nil
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
