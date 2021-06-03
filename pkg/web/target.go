package web

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"

	uuid "github.com/satori/go.uuid"
	"goji.io/pat"
)

type targetCreateParam struct {
	datastore.Target

	RunnerUser    string `json:"runner_user"`
	GHEDomain     string `json:"ghe_domain"`
	RunnerVersion string `json:"runner_version"`
	ProviderURL   string `json:"provider_url"`
}

// function pointer (for testing)
var (
	GHExistGitHubRepositoryFunc = gh.ExistGitHubRepository
	GHListRunnersFunc           = gh.ListRunners
	GHIsInstalledGitHubApp      = gh.IsInstalledGitHubApp
	GHGenerateGitHubAppsToken   = gh.GenerateGitHubAppsToken
)

func toNullString(input string) sql.NullString {
	if input == "" {
		return sql.NullString{
			Valid: false,
		}
	}

	return sql.NullString{
		Valid:  true,
		String: input,
	}
}

func isValidTargetCreateParam(input targetCreateParam) (bool, error) {
	if input.Scope == "" || input.ResourceType == datastore.ResourceTypeUnknown {
		return false, fmt.Errorf("scope, resource_type must be set")
	}

	if input.GHEDomain != "" {
		if _, err := url.Parse(input.GHEDomain); err != nil {
			return false, fmt.Errorf("domain of GitHub Enterprise is not valid URL: %w", err)
		}
	}

	if input.RunnerVersion != "" {
		// valid format: vX.X.X (X is [0-9])
		if !strings.HasPrefix(input.RunnerVersion, "v") {
			return false, fmt.Errorf("runner_version must has prefix 'v'")
		}

		s := strings.Split(input.RunnerVersion, ".")
		if len(s) != 3 {
			return false, fmt.Errorf("runner_version must has version of major, sem, patch")
		}
	}

	return true, nil
}

func (t *targetCreateParam) toDS(appToken string, tokenExpired time.Time) datastore.Target {
	gheDomain := toNullString(t.GHEDomain)
	runnerUser := toNullString(t.RunnerUser)
	runnerVersion := toNullString(t.RunnerVersion)
	providerURL := toNullString(t.ProviderURL)

	return datastore.Target{
		UUID:           t.UUID,
		Scope:          t.Scope,
		GitHubToken:    appToken,
		TokenExpiredAt: tokenExpired,
		GHEDomain:      gheDomain,
		ResourceType:   t.ResourceType,
		RunnerUser:     runnerUser,
		RunnerVersion:  runnerVersion,
		ProviderURL:    providerURL,
	}
}

func handleTargetList(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	ctx := r.Context()

	ts, err := datastore.ListTargets(ctx, ds)
	if err != nil {
		logger.Logf(false, "failed to retrieve list of target: %+v", err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore read error")
	}

	var targets []datastore.Target
	for _, t := range ts {
		target := sanitizeTarget(&t)
		targets = append(targets, *target)
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(targets)
	return
}

func handleTargetRead(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	ctx := r.Context()
	targetID, err := parseReqTargetID(r)
	if err != nil {
		logger.Logf(false, "failed to decode request body: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "incorrect target id")
		return
	}

	target, err := ds.GetTarget(ctx, targetID)
	if err != nil {
		logger.Logf(false, "failed to retrieve target from datastore: %+v", err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore read error")
		return
	}

	target = sanitizeTarget(target)

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(target)
	return
}

func sanitizeTarget(t *datastore.Target) *datastore.Target {
	t.GitHubToken = ""

	return t
}

func handleTargetUpdate(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	outputErrorMsg(w, http.StatusMethodNotAllowed, "not implement")
	return
}

func handleTargetDelete(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	ctx := r.Context()
	targetID, err := parseReqTargetID(r)
	if err != nil {
		logger.Logf(false, "failed to decode request body: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "incorrect target id")
		return
	}

	target, err := ds.GetTarget(ctx, targetID)
	if err != nil {
		logger.Logf(false, "failed to get target: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "incorrect target id (not found)")
		return
	}
	switch target.Status {
	case datastore.TargetStatusRunning:
		logger.Logf(true, "%s is running now", targetID)
		outputErrorMsg(w, http.StatusBadRequest, "target has running runner now, please stop all runner")
		return
	case datastore.TargetStatusDeleted:
		outputErrorMsg(w, http.StatusBadRequest, "target is already deleted")
		return
	}

	if err := ds.DeleteTarget(ctx, targetID); err != nil {
		logger.Logf(false, "failed to delete target in datastore: %+v", err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore delete error")
		return
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusNoContent)
	return
}

func parseReqTargetID(r *http.Request) (uuid.UUID, error) {
	targetIDStr := pat.Param(r, "id")
	targetID, err := uuid.FromString(targetIDStr)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to parse target id: %w", err)
	}

	return targetID, nil
}

func outputErrorMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	w.WriteHeader(status)

	json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: msg})
}
