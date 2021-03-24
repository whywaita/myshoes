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

// GHExistGitHubRepositoryFunc is function pointer of gh.ExistGitHubRepository (for testing)
var GHExistGitHubRepositoryFunc = gh.ExistGitHubRepository

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
	if input.Scope == "" || input.GitHubPersonalToken == "" || input.ResourceType == datastore.ResourceTypeUnknown {
		return false, fmt.Errorf("scope, github_personal_token, resource_type must be set")
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

func (t *targetCreateParam) toDS() datastore.Target {
	gheDomain := toNullString(t.GHEDomain)
	runnerUser := toNullString(t.RunnerUser)
	runnerVersion := toNullString(t.RunnerVersion)
	providerURL := toNullString(t.ProviderURL)

	return datastore.Target{
		UUID:                t.UUID,
		Scope:               t.Scope,
		GitHubPersonalToken: t.GitHubPersonalToken,
		GHEDomain:           gheDomain,
		ResourceType:        t.ResourceType,
		RunnerUser:          runnerUser,
		RunnerVersion:       runnerVersion,
		ProviderURL:         providerURL,
	}
}

func handleTargetCreate(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	// input values: scope, gpt, ghe_domain, resource_type
	ctx := r.Context()
	inputTarget := targetCreateParam{}

	if err := json.NewDecoder(r.Body).Decode(&inputTarget); err != nil {
		logger.Logf(false, "failed to decode request body: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "json decode error")
		return
	}

	if ok, err := isValidTargetCreateParam(inputTarget); !ok {
		logger.Logf(false, "failed to validate input: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, err.Error())
		return
	}
	t := inputTarget.toDS()

	if err := GHExistGitHubRepositoryFunc(t.Scope, t.GHEDomain.String, t.GHEDomain.Valid, t.GitHubPersonalToken); err != nil {
		logger.Logf(false, "failed to found github repository: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "github scope is invalid (maybe, repository is not found)")
		return
	}

	t.UUID = uuid.NewV4()
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	if err := ds.CreateTarget(ctx, t); err != nil {
		logger.Logf(false, "failed to create target in datastore: %+v", err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore create error")
		return
	}
	target, err := ds.GetTarget(ctx, t.UUID)
	if err != nil {
		logger.Logf(false, "failed to get recently target in datastore: %+v", err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore get error")
		return
	}

	sanitized := sanitizeTarget(target)

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sanitized)
	return
}

func handleTargetList(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	ctx := r.Context()

	ts, err := ds.ListTargets(ctx)
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
	t.GitHubPersonalToken = ""

	return t
}

func handleTargetUpdate(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {}

func handleTargetDelete(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	ctx := r.Context()
	targetID, err := parseReqTargetID(r)
	if err != nil {
		logger.Logf(false, "failed to decode request body: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "incorrect target id")
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
