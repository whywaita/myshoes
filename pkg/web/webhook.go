package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/v80/github"
	uuid "github.com/satori/go.uuid"

	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
	"github.com/whywaita/myshoes/pkg/metric"
)

// HandleGitHubEvent handle GitHub webhook event
func HandleGitHubEvent(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	ctx := r.Context()
	startTime := time.Now()
	eventType := github.WebHookType(r)

	payload, err := github.ValidatePayload(r, config.Config.GitHub.AppSecret)
	if err != nil {
		logger.Logf(false, "failed to validate webhook payload: %+v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		metric.WebhookReceivedTotal.WithLabelValues(eventType, "invalid", "unknown").Inc()
		return
	}
	webhookEvent, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		logger.Logf(false, "failed to parse webhook payload: %+v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		metric.WebhookReceivedTotal.WithLabelValues(eventType, "parse_error", "unknown").Inc()
		return
	}

	// Extract runs-on labels
	runsOn := "unknown"
	if eventType == "workflow_job" {
		labels, err := gh.ExtractRunsOnLabels(payload)
		if err == nil && len(labels) > 0 {
			runsOn = strings.Join(labels, ",")
		}
	}

	switch event := webhookEvent.(type) {
	case *github.PingEvent:
		if err := receivePingWebhook(ctx, event); err != nil {
			logger.Logf(false, "failed to process ping event: %+v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			metric.WebhookReceivedTotal.WithLabelValues("ping", "error", "n/a").Inc()
			return
		}

		w.WriteHeader(http.StatusOK)
		metric.WebhookReceivedTotal.WithLabelValues("ping", "success", "n/a").Inc()
		metric.WebhookProcessingDuration.WithLabelValues("ping", "n/a").Observe(time.Since(startTime).Seconds())
		return
	case *github.CheckRunEvent:
		if !config.Config.ModeWebhookType.Equal("check_run") {
			logger.Logf(false, "receive CheckRunEvent, but set %s. So ignore", config.Config.ModeWebhookType)
			return
		}

		if err := receiveCheckRunWebhook(ctx, event, ds); err != nil {
			logger.Logf(false, "failed to process check_run event: %+v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			metric.WebhookReceivedTotal.WithLabelValues("check_run", "error", "n/a").Inc()
			return
		}

		w.WriteHeader(http.StatusOK)
		metric.WebhookReceivedTotal.WithLabelValues("check_run", "success", "n/a").Inc()
		metric.WebhookProcessingDuration.WithLabelValues("check_run", "n/a").Observe(time.Since(startTime).Seconds())
		return
	case *github.WorkflowJobEvent:
		if !config.Config.ModeWebhookType.Equal("workflow_job") {
			logger.Logf(false, "receive WorkflowJobEvent, but set %s. So ignore", config.Config.ModeWebhookType)
			return
		}

		if err := receiveWorkflowJobWebhook(ctx, event, ds); err != nil {
			logger.Logf(false, "failed to process workflow_job event: %+v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			metric.WebhookReceivedTotal.WithLabelValues("workflow_job", "error", runsOn).Inc()
			return
		}

		w.WriteHeader(http.StatusOK)
		metric.WebhookReceivedTotal.WithLabelValues("workflow_job", "success", runsOn).Inc()
		metric.WebhookProcessingDuration.WithLabelValues("workflow_job", runsOn).Observe(time.Since(startTime).Seconds())
		return
	default:
		logger.Logf(false, "receive not register event(%+v), return NotFound", event)
		w.WriteHeader(http.StatusNotFound)
		metric.WebhookReceivedTotal.WithLabelValues(eventType, "not_found", "unknown").Inc()
		return
	}
}

func receivePingWebhook(_ context.Context, event *github.PingEvent) error {
	// do nothing
	return nil
}

func receiveCheckRunWebhook(ctx context.Context, event *github.CheckRunEvent, ds datastore.Datastore) error {
	action := event.GetAction()
	installationID := event.GetInstallation().GetID()

	repo := event.GetRepo()
	repoName := repo.GetFullName()
	repoURL := repo.GetHTMLURL()

	if action != "created" {
		logger.Logf(true, "check_action is not created, ignore (%s)", action)
		return nil
	}

	jb, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to json.Marshal: %w", err)
	}
	if err := processCheckRun(ctx, ds, repoName, repoURL, installationID, jb); err != nil {
		return err
	}

	// Record job enqueued metric
	metric.WebhookJobsEnqueued.WithLabelValues("check_run", repoName, "n/a").Inc()

	return nil
}

// processCheckRun process webhook event
// repoName is :owner/:repo
// repoURL is https://github.com/:owenr/:repo (in github.com) or https://github.example.com/:owner/:repo (in GitHub Enterprise)
func processCheckRun(ctx context.Context, ds datastore.Datastore, repoName, repoURL string, installationID int64, requestJSON []byte) error {
	if err := gh.CheckSignature(installationID); err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("failed to parse repository url from event: %w", err)
	}
	//var domain string
	gheDomain := ""
	if u.Host != "github.com" {
		gheDomain = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	logger.Logf(false, "receive webhook repository: %s/%s", gheDomain, repoName)
	target, err := datastore.SearchRepo(ctx, ds, repoName)
	if err != nil {
		return fmt.Errorf("failed to search registered target: %w", err)
	}

	if !target.CanReceiveJob() {
		// do nothing if status is cannot receive
		logger.Logf(false, "%s/%s is %s now, do nothing", gheDomain, repoName, target.Status)
		return nil
	}

	jobID := uuid.NewV4()
	j := datastore.Job{
		UUID: jobID,
		GHEDomain: sql.NullString{
			String: gheDomain,
			Valid:  gheDomain != "",
		},
		Repository:     repoName,
		CheckEventJSON: string(requestJSON),
		TargetID:       target.UUID,
	}
	if err := ds.EnqueueJob(ctx, j); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

func receiveWorkflowJobWebhook(ctx context.Context, event *github.WorkflowJobEvent, ds datastore.Datastore) error {
	action := event.GetAction()
	installationID := event.GetInstallation().GetID()

	repo := event.GetRepo()
	repoName := repo.GetFullName()
	repoURL := repo.GetHTMLURL()

	labels := event.GetWorkflowJob().Labels
	if !gh.IsRequestedMyshoesLabel(labels) {
		// is not request myshoes, So will be ignored
		logger.Logf(true, "label \"myshoes\" is not found in labels, so ignore (labels: %s)", labels)
		return nil
	}

	if action != "queued" {
		logger.Logf(true, "workflow_job actions is not queued, ignore")
		return nil
	}

	jb, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to json.Marshal: %w", err)
	}

	if err := processCheckRun(ctx, ds, repoName, repoURL, installationID, jb); err != nil {
		return err
	}

	// Record job enqueued metric for workflow_job
	runsOn := strings.Join(labels, ",")
	metric.WebhookJobsEnqueued.WithLabelValues("workflow_job", repoName, runsOn).Inc()

	return nil
}
