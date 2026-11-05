package gh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/whywaita/myshoes/pkg/config"

	"github.com/google/go-github/v80/github"
)

// function pointers (for testing)
var (
	GHlistInstallations     = listInstallations
	GHlistAppsInstalledRepo = listAppsInstalledRepo
)

// GenerateGitHubAppsToken generate token of GitHub Apps using private key
// clientApps needs to response of `NewClientGitHubApps()`
func GenerateGitHubAppsToken(ctx context.Context, clientApps *github.Client, installationID int64, scope string) (string, *time.Time, error) {
	token, resp, err := clientApps.Apps.CreateInstallationToken(ctx, installationID, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate token from API: %w", err)
	}
	storeRateLimit(scope, resp.Rate)
	expiresAt := token.ExpiresAt.GetTime()
	return *token.Token, expiresAt, nil
}

// IsInstalledGitHubApp check installed GitHub Apps in gheDomain + inputScope
// clientApps needs to response of `NewClientGitHubApps()`
func IsInstalledGitHubApp(ctx context.Context, inputScope string) (int64, error) {
	installations, err := GHlistInstallations(ctx)
	if err != nil {
		return -1, fmt.Errorf("failed to get list of installations: %w", err)
	}

	// inputScope is "owner" or "owner/repo".
	// e.g.) `https://github.com/example/sample` -> `example/sample`
	owner, _ := DivideScope(inputScope)

	for _, i := range installations {
		// GitHub API may return null elements in array responses,
		// and they are decoded into nil pointers.
		if i == nil || i.SuspendedAt != nil {
			continue
		}

		login := i.GetAccount().GetLogin()
		id := i.GetID()
		if login == "" || id == 0 {
			continue
		}

		// login is username or Organization name.
		// Compare the owner exactly (GitHub logins are case-insensitive),
		// so that "example" does not match "example-other/repo".
		if strings.EqualFold(owner, login) {
			switch {
			case strings.EqualFold(i.GetRepositorySelection(), "all"):
				// "all" can use GitHub Apps in all repositories that joined login.
				return id, nil
			case strings.EqualFold(i.GetRepositorySelection(), "selected"):
				// "selected" can use GitHub Apps in only some repositories that permitted.
				// So, need to check more using other endpoint.
				err := isInstalledGitHubAppSelected(ctx, inputScope, id)
				if err == nil {
					// found
					return id, nil
				}
			}
		}
	}

	return -1, &ErrIsNotInstalledGitHubApps{
		githubURL:  config.Config.GitHubURL,
		inputScope: inputScope,
	}
}

type ErrIsNotInstalledGitHubApps struct {
	githubURL  string
	inputScope string
}

func (e *ErrIsNotInstalledGitHubApps) Error() string {
	return fmt.Sprintf("%s/%s is not installed configured GitHub Apps", e.githubURL, e.inputScope)
}

func (e *ErrIsNotInstalledGitHubApps) Unwrap() error {
	return fmt.Errorf("%s", e.Error())
}

func isInstalledGitHubAppSelected(ctx context.Context, inputScope string, installationID int64) error {
	installedRepository, err := GHlistAppsInstalledRepo(ctx, installationID)
	if err != nil {
		return fmt.Errorf("failed to get list of installed repositories: %w", err)
	}

	if len(installedRepository) <= 0 {
		return fmt.Errorf("installed repository is not found")
	}

	switch DetectScope(inputScope) {
	case Organization:
		// Scope is Organization and installed repository is existed
		// So GitHub Apps installed
		return nil
	case Repository:
		for _, repo := range installedRepository {
			if strings.EqualFold(repo.GetFullName(), inputScope) {
				return nil
			}
		}

		return fmt.Errorf("not found")
	default:
		return fmt.Errorf("%s can't detect scope", inputScope)
	}
}
