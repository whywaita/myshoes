package web

import (
	"net/http"

	"github.com/whywaita/myshoes/pkg/datastore"

	"github.com/whywaita/myshoes/pkg/metric"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func newMetricsHandler(ds datastore.Datastore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		registry := prometheus.NewRegistry()
		registry.MustRegister(metric.NewCollector(ctx, ds))

		gatherers := prometheus.Gatherers{
			prometheus.DefaultGatherer,
			registry,
		}
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}
