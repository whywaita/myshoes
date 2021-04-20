package web

import (
	"encoding/json"
	"net/http"

	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/logger"
)

type inputConfig struct {
	Debug bool `json:"debug"`
}

func handleConfigDebug(w http.ResponseWriter, r *http.Request) {
	i := inputConfig{}

	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		logger.Logf(false, "failed to decode request body: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "json decode error")
		return
	}

	config.Config.Debug = i.Debug
	logger.Logf(false, "switch debug mode to %t", i.Debug)
	w.WriteHeader(http.StatusNoContent)
	return
}
