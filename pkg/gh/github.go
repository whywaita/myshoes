package gh

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v35/github"
	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/logger"
	"golang.org/x/oauth2"
)

var (
	// ErrNotFound is error for not found
	ErrNotFound = fmt.Errorf("not found")

	// ResponseCache is cache variable
	responseCache *cache.Cache
)

func init() {
	c := cache.New(5*time.Minute, 10*time.Minute)
	responseCache = c
}

// NewClient create a client of GitHub
func NewClient(ctx context.Context, personalToken, gheDomain string) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: personalToken,
		})
	tc := oauth2.NewClient(ctx, ts)

	if gheDomain == "" {
		return github.NewClient(tc), nil
	}

	return github.NewEnterpriseClient(gheDomain, gheDomain, tc)
}

// NewClientGitHubApps create a client of GitHub using Private Key from GitHub Apps
// docs: https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-a-github-app
func NewClientGitHubApps(gheDomain string) (*github.Client, error) {
	appID := config.Config.GitHub.AppID
	pem := config.Config.GitHub.PEMByte

	tr := http.DefaultTransport
	itr, err := ghinstallation.NewAppsTransport(tr, appID, pem)
	if err != nil {
		return nil, fmt.Errorf("failed to create Apps transport: %w", err)
	}

	if gheDomain == "" {
		return github.NewClient(&http.Client{Transport: itr}), nil
	}
	itr.BaseURL = gheDomain
	return github.NewEnterpriseClient(gheDomain, gheDomain, &http.Client{Transport: itr})
}

// NewClientInstallation create a client of Github using installation ID from GitHub Apps
// docs: https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-an-installation
func NewClientInstallation(gheDomain string, installationID int64) (*github.Client, error) {
	appID := config.Config.GitHub.AppID
	pem := config.Config.GitHub.PEMByte

	tr := http.DefaultTransport
	itr, err := ghinstallation.New(tr, appID, installationID, pem)
	if err != nil {
		return nil, fmt.Errorf("failed to create Apps transport: %w", err)
	}

	if gheDomain == "" {
		return github.NewClient(&http.Client{Transport: itr}), nil
	}
	itr.BaseURL = gheDomain
	return github.NewEnterpriseClient(gheDomain, gheDomain, &http.Client{Transport: itr})
}

// CheckSignature check trust installation id from event.
func CheckSignature(installationID int64) error {
	appID := config.Config.GitHub.AppID
	pem := config.Config.GitHub.PEMByte

	tr := http.DefaultTransport
	_, err := ghinstallation.New(tr, appID, installationID, pem)
	if err != nil {
		return fmt.Errorf("failed to create GitHub installation: %w", err)
	}

	return nil
}

// ExistGitHubRepository check exist of github repository
func ExistGitHubRepository(scope, gheDomain string, githubPersonalToken string) error {
	repoURL, err := getRepositoryURL(scope, gheDomain)
	if err != nil {
		return fmt.Errorf("failed to get repository url: %w", err)
	}

	client := http.DefaultClient
	req, err := http.NewRequest(http.MethodGet, repoURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("token %s", githubPersonalToken))

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

// ExistGitHubRunner check exist registered of github runner
func ExistGitHubRunner(ctx context.Context, client *github.Client, owner, repo, runnerName string) (*github.Runner, error) {
	runners, err := ListRunners(ctx, client, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of runners: %w", err)
	}

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
		PerPage: 10,
	}

	var rs []*github.Runner
	for {
		logger.Logf(true, "get runners from GitHub, page: %d, now all runners: %d", opts.Page, len(rs))
		runners, resp, err := listRunners(ctx, client, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list runners: %w", err)
		}

		rs = append(rs, runners.Runners...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	responseCache.Set(getCacheKey(owner, repo), rs, 1*time.Second)
	logger.Logf(true, "found %d runners", len(rs))

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
