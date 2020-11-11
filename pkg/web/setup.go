package web

import "net/http"

const (
	DefaultDistribution = "ubuntu"
)

// handleSetup return setup bash scripts
func handleSetup(w http.ResponseWriter, r *http.Request) {
	distribution := r.FormValue("os")
	if distribution == "" {
		distribution = DefaultDistribution
	}
}
