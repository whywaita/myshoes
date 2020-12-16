package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/whywaita/myshoes/pkg/datastore"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/logger"

	goji "goji.io"
	"goji.io/pat"
)

// Serve start webhook receiver
func Serve(ds datastore.Datastore) error {
	mux := goji.NewMux()

	mux.HandleFunc(pat.Get("/healthz"), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		w.WriteHeader(http.StatusOK)

		h := struct {
			Health string `json:"health"`
		}{
			Health: "ok",
		}

		json.NewEncoder(w).Encode(h)
		return
	})

	mux.HandleFunc(pat.Post("/github/events"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleGitHubEvent(w, r, ds)
	})

	// REST API for targets
	mux.HandleFunc(pat.Post("/target"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleTargetCreate(w, r, ds)
	})
	mux.HandleFunc(pat.Get("/target/:id"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleTargetRead(w, r, ds)
	})
	mux.HandleFunc(pat.Put("/target/:id"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleTargetUpdate(w, r, ds)
	})
	mux.HandleFunc(pat.Delete("/target/:id"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleTargetDelete(w, r, ds)
	})

	listenAddress := fmt.Sprintf(":%d", config.Config.Port)
	logger.Logf(false, "start webhook receiver, listen %s", listenAddress)
	if err := http.ListenAndServe(listenAddress, mux); err != nil {
		return fmt.Errorf("failed to listen and serve: %w", err)
	}

	return nil
}

func apacheLogging(r *http.Request) {
	t := time.Now().UTC()
	logger.Logf(false, "HTTP - %s - - %s \"%s %s %s\"\n",
		r.RemoteAddr,
		t.Format("02/Jan/2006:15:04:05 -0700"),
		r.Method,
		r.URL.Path,
		r.Proto,
		//interceptor.HTTPStatus,
		//interceptor.ResponseSize,
		//r.UserAgent(),
		//time.Since(t),
	)
}
