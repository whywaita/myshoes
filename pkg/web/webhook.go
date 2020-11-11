package web

import (
	"context"
	"fmt"
	"net/http"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/logger"

	"github.com/whywaita/myshoes/pkg/gh"

	"github.com/google/go-github/v32/github"
)

func handleGitHubEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := github.ValidatePayload(r, config.Config.GitHub.AppSecret)
	if err != nil {
		logger.Logf("failed to validate webhook payload: %+v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	webhookEvent, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		logger.Logf("failed to parse webhook payload: %+v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch event := webhookEvent.(type) {
	case *github.CheckRunEvent:
		if err := processEvent(ctx, event); err != nil {
			logger.Logf("failed to process event: %+v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func processEvent(ctx context.Context, event *github.CheckRunEvent) error {
	if event.GetAction() != "created" {
		return nil
	}

	installationID := event.GetInstallation().GetID()
	//client, err := newGitHubClient(installationID)
	_, err := gh.NewClient(installationID)
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// TODO: process action => create runner
	return nil
}
