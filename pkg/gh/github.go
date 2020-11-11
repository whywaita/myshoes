package gh

import (
	"fmt"
	"net/http"
	"time"

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
