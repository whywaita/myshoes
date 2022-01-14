package gh

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	cacheRegistrationToken = cache.New(1*time.Hour, 1*time.Hour)
)

// GetRunnerRegistrationToken get token for register runner
// clientInstallation needs to response of `NewClientInstallation()`
func GetRunnerRegistrationToken(ctx context.Context, gheDomain string, installationID int64, scope string) (string, error) {
	cachedToken := getRunnerRegisterTokenFromCache(installationID)
	if cachedToken != "" {
		return cachedToken, nil
	}

	rrToken, expiresAt, err := generateRunnerRegisterToken(ctx, gheDomain, installationID, scope)
	if err != nil {
		return "", fmt.Errorf("failed to generate runner register token: %w", err)
	}
	setRunnerRegisterTokenCache(installationID, rrToken, *expiresAt)
	return rrToken, nil
}

// generateRunnerRegistrationToken generate token for register runner
// clientInstallation needs to response of `NewClientInstallation()`
func generateRunnerRegisterToken(ctx context.Context, gheDomain string, installationID int64, scope string) (string, *time.Time, error) {
	clientInstallation, err := NewClientInstallation(gheDomain, installationID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create a client installation: %w", err)
	}

	switch DetectScope(scope) {
	case Organization:
		token, _, err := clientInstallation.Actions.CreateOrganizationRegistrationToken(ctx, scope)
		if err != nil {
			return "", nil, fmt.Errorf("failed to generate registration token for organization (scope: %s): %w", scope, err)
		}
		return *token.Token, &token.ExpiresAt.Time, nil
	case Repository:
		owner, repo := DivideScope(scope)
		token, _, err := clientInstallation.Actions.CreateRegistrationToken(ctx, owner, repo)
		if err != nil {
			return "", nil, fmt.Errorf("failed to generate registration token for repository (scope: %s): %w", scope, err)
		}
		return *token.Token, &token.ExpiresAt.Time, nil
	default:
		return "", nil, fmt.Errorf("failed to detect scope (scope: %s)", scope)
	}
}

func setRunnerRegisterTokenCache(installationID int64, token string, expiresAt time.Time) {
	expiresDuration := time.Until(expiresAt.Add(-time.Minute))

	cacheRegistrationToken.Set(strconv.FormatInt(installationID, 10), token, expiresDuration)
}

func getRunnerRegisterTokenFromCache(installationID int64) string {
	got, found := cacheRegistrationToken.Get(strconv.FormatInt(installationID, 10))
	if !found {
		return ""
	}
	token, ok := got.(string)
	if !ok {
		return ""
	}
	return token
}
