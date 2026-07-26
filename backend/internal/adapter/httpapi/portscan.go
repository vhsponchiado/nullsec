package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/usecase/portscan"
)

func (a *API) portscanPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"top100":  portscan.Top100,
		"top1000": portscan.Top1000,
	})
}

func (a *API) portscanStart(w http.ResponseWriter, r *http.Request) {
	var req portscan.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	id, total, err := a.portscan.Start(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"scan_id": id, "total": total})
}

func (a *API) portscanStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.portscan.Stop(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (a *API) portscanStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, total, results, ok := a.portscan.Snapshot(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"status": status, "total": total, "results": results})
}
