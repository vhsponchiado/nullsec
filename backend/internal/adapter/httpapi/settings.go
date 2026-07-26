package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/entity"
)

func (a *API) getUpstreamProxy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.settings.GetUpstreamProxy())
}

func (a *API) setUpstreamProxy(w http.ResponseWriter, r *http.Request) {
	var cfg entity.UpstreamProxyConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	updated, err := a.settings.SetUpstreamProxy(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, updated)
}
