package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/usecase/sqliscan"
)

func (a *API) sqliscanStart(w http.ResponseWriter, r *http.Request) {
	var req sqliscan.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	id, total, err := a.sqliscan.Start(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"scan_id": id, "total": total})
}

func (a *API) sqliscanStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.sqliscan.Stop(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (a *API) sqliscanStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, total, results, ok := a.sqliscan.Snapshot(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"status": status, "total": total, "results": results})
}
