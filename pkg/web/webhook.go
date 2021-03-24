package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v32/github"
	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
)

func handleGitHubEvent(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	ctx := r.Context()

	payload, err := github.ValidatePayload(r, config.Config.GitHub.AppSecret)
	if err != nil {
		logger.Logf(false, "failed to validate webhook payload: %+v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	webhookEvent, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		logger.Logf(false, "failed to parse webhook payload: %+v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch event := webhookEvent.(type) {
	case *github.PingEvent:
		if err := receivePingWebhook(ctx, event); err != nil {
			logger.Logf(false, "failed to process ping event: %+v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	case *github.CheckRunEvent:
		if err := receiveCheckRunWebhook(ctx, event, ds); err != nil {
			logger.Logf(false, "failed to process check_run event: %+v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	default:
		logger.Logf(false, "receive not register event(%+v), return NotFound", event)
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

func receivePingWebhook(ctx context.Context, event *github.PingEvent) error {
	return nil
}

func receiveCheckRunWebhook(ctx context.Context, event *github.CheckRunEvent, ds datastore.Datastore) error {
	action := event.GetAction()
	installationID := event.GetInstallation().GetID()

	repoName := *(event.Repo.FullName)
	repoURL := *(event.Repo.HTMLURL)

	jb, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to json.Marshal: %w", err)
	}

	return processCheckRun(ctx, ds, action, repoName, repoURL, installationID, jb)
}

// processCheckRun process webhook event
// repoName is :owner/:repo
// repoURL is https://github.com/:owenr/:repo (in github.com) or https://github.example.com/:owner/:repo (in GitHub Enterprise)
func processCheckRun(ctx context.Context, ds datastore.Datastore, checkAction, repoName, repoURL string, installationID int64, requestJSON []byte) error {
	if checkAction != "created" {
		logger.Logf(true, "check_action is not created, ignore")
		return nil
	}

	if err := gh.CheckSignature(installationID); err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("failed to parse repository url from event: %w", err)
	}
	var domain string
	gheDomain := ""
	if u.Host != "github.com" {
		gheDomain = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		domain = gheDomain
	} else {
		domain = "https://github.com"
	}

	logger.Logf(false, "receive webhook repository: %s/%s", domain, repoName)
	target, err := searchRepo(ctx, ds, gheDomain, repoName)
	if err != nil {
		return fmt.Errorf("failed to search registered target: %w", err)
	}

	if !target.CanReceiveJob() {
		// do nothing if status is cannot receive
		logger.Logf(false, "%s/%s is %s now, do nothing", target.Status, domain, repoName)
		return nil
	}

	jobID := uuid.NewV4()
	var jobDomain sql.NullString
	if gheDomain == "" {
		jobDomain = sql.NullString{
			Valid: false,
		}
	} else {
		jobDomain = sql.NullString{
			String: gheDomain,
			Valid:  true,
		}
	}

	j := datastore.Job{
		UUID:           jobID,
		GHEDomain:      jobDomain,
		Repository:     repoName,
		CheckEventJSON: string(requestJSON),
		TargetID:       target.UUID,
	}
	if err := ds.EnqueueJob(ctx, j); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// searchRepo search datastore.Target from datastore
// format of repo is "orgs/repos"
func searchRepo(ctx context.Context, ds datastore.Datastore, gheDomain, repo string) (*datastore.Target, error) {
	sep := strings.Split(repo, "/")
	if len(sep) != 2 {
		return nil, fmt.Errorf("incorrect repo format ex: orgs/repo")
	}

	// use repo scope if set repo
	repoTarget, err := ds.GetTargetByScope(ctx, gheDomain, repo)
	if err == nil {
		return repoTarget, nil
	} else if err != datastore.ErrNotFound {
		return nil, fmt.Errorf("failed to get target from repo: %w", err)
	}

	// repo is not found, so search org target
	org := sep[0]
	orgTarget, err := ds.GetTargetByScope(ctx, gheDomain, org)
	if err != nil {
		return nil, fmt.Errorf("failed to get target from organization: %w", err)
	}

	return orgTarget, nil
}
