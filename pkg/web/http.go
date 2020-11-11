package web

import (
	"fmt"
	"net/http"

	"github.com/whywaita/myshoes/internal/config"

	"github.com/whywaita/myshoes/pkg/logger"
)

// Serve start webhook receiver
func Serve() error {
	http.HandleFunc("/github/events", handleGitHubEvent)
	http.HandleFunc("/setup", handleSetup)

	logger.Logf("start webhook receiver")
	if err := http.ListenAndServe(":"+string(config.Config.Port), nil); err != nil {
		return fmt.Errorf("failed to listen and serve: %w", err)
	}

	return nil
}
