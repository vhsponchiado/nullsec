package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/usecase/spider"
)

func (a *API) spiderStart(w http.ResponseWriter, r *http.Request) {
	var req spider.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	id, err := a.spider.Start(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"spider_id": id})
}

func (a *API) spiderStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.spider.Stop(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (a *API) spiderStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, pages, ok := a.spider.Snapshot(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"status": status, "pages": pages})
}
