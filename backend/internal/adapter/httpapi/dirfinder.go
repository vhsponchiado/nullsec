package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/usecase/dirfinder"
)

func (a *API) dirfinderWordlistSize(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]int{"size": len(dirfinder.Wordlist)})
}

func (a *API) dirfinderStart(w http.ResponseWriter, r *http.Request) {
	var req dirfinder.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	id, total, err := a.dirfinder.Start(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"scan_id": id, "total": total})
}

func (a *API) dirfinderStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.dirfinder.Stop(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (a *API) dirfinderStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, total, results, ok := a.dirfinder.Snapshot(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"status": status, "total": total, "results": results})
}
