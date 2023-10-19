package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/logger"

	goji "goji.io"
	"goji.io/pat"
)

// NewMux create routed mux
func NewMux(ds datastore.Datastore) *goji.Mux {
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
	mux.HandleFunc(pat.Get("/target"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleTargetList(w, r, ds)
	})
	mux.HandleFunc(pat.Get("/target/:id"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleTargetRead(w, r, ds)
	})
	mux.HandleFunc(pat.Post("/target/:id"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleTargetUpdate(w, r, ds)
	})
	mux.HandleFunc(pat.Delete("/target/:id"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleTargetDelete(w, r, ds)
	})

	// Config endpoints
	mux.HandleFunc(pat.Post("/config/debug"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleConfigDebug(w, r)
	})
	mux.HandleFunc(pat.Post("/config/strict"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleConfigStrict(w, r)
	})

	// metrics endpoint
	mux.HandleFunc(pat.Get("/metrics"), func(w http.ResponseWriter, r *http.Request) {
		apacheLogging(r)
		handleMetrics(w, r, ds)
	})

	return mux
}

// Serve start webhook receiver
func Serve(ctx context.Context, ds datastore.Datastore) error {
	mux := NewMux(ds)
	listenAddress := fmt.Sprintf(":%d", config.Config.Port)
	s := &http.Server{
		Addr:    listenAddress,
		Handler: mux,
	}

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		logger.Logf(false, "start webhook receiver, listen %s", listenAddress)
		if err := s.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("failed to listen and serve: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown(ctx)
	case err := <-errCh:
		return fmt.Errorf("occurred error in web serve: %w", err)
	}
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
