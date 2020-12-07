package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/whywaita/myshoes/pkg/logger"

	"github.com/whywaita/myshoes/pkg/gh"

	"goji.io/pat"

	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/pkg/datastore"
)

func handleTargetCreate(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	// input values: scope, gpt, ghe_domain, resource_type
	ctx := context.Background()
	t := datastore.Target{}

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		logger.Logf("%+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "json decode error")
		return
	}

	// TODO: input validate

	if err := gh.ExistGitHubRepository(t.Scope, t.GHEDomain.String, t.GHEDomain.Valid, t.GitHubPersonalToken); err != nil {
		logger.Logf("%+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "github scope is invalid (maybe, repository is not found)")
		return
	}

	t.UUID = uuid.NewV4()
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	if err := ds.CreateTarget(ctx, t); err != nil {
		logger.Logf("%+v", err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore create error")
		return
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
	return
}

func handleTargetRead(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	ctx := context.Background()
	targetID, err := parseReqTargetID(r)
	if err != nil {
		logger.Logf("%+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "incorrect target id")
	}

	target, err := ds.GetTarget(ctx, targetID)
	if err != nil {
		logger.Logf("%+v", err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore read error")
		return
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(target)
	return
}

func handleTargetUpdate(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {}

func handleTargetDelete(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	ctx := context.Background()
	targetID, err := parseReqTargetID(r)
	if err != nil {
		logger.Logf("%+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "incorrect target id")
		return
	}

	if err := ds.DeleteTarget(ctx, targetID); err != nil {
		logger.Logf("%+v", err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore delete error")
		return
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
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
