package web

import "net/http"

const (
	DefaultDistribution = "ubuntu"
)

// handleRunnerSetup return setup bash scripts
func handleRunnerSetup(w http.ResponseWriter, r *http.Request) {
	// TODO: 来たリクエストがどのリクエストなのかを判定する、ホスト名とか？UUID？
	// TODO: https://github.com/actions/runner/blob/main/scripts/create-latest-svc.sh
}
