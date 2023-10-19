package web

import (
	"encoding/json"
	"net/http"

	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/logger"
)

type inputConfigDebug struct {
	Debug bool `json:"debug"`
}

type inputConfigStrict struct {
	Strict bool `json:"strict"`
}

func handleConfigDebug(w http.ResponseWriter, r *http.Request) {
	i := inputConfigDebug{}

	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		logger.Logf(false, "failed to decode request body: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "json decode error")
		return
	}

	config.Config.Debug = i.Debug
	logger.Logf(false, "switch debug mode to %t", i.Debug)
	w.WriteHeader(http.StatusNoContent)
}

func handleConfigStrict(w http.ResponseWriter, r *http.Request) {
	i := inputConfigStrict{}

	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		logger.Logf(false, "failed to decode request body: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "json decode error")
		return
	}

	config.Config.Strict = i.Strict
	logger.Logf(false, "switch strict mode to %t", i.Strict)
	w.WriteHeader(http.StatusNoContent)
}
