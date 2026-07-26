package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/usecase/xssscan"
)

func (a *API) xssscanStart(w http.ResponseWriter, r *http.Request) {
	var req xssscan.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	id, total, err := a.xssscan.Start(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"scan_id": id, "total": total})
}

func (a *API) xssscanStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.xssscan.Stop(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (a *API) xssscanStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, total, results, ok := a.xssscan.Snapshot(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"status": status, "total": total, "results": results})
}
