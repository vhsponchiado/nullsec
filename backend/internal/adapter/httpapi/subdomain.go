package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/usecase/subdomain"
)

func (a *API) subdomainWordlistSize(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]int{"size": len(subdomain.Wordlist)})
}

func (a *API) subdomainStart(w http.ResponseWriter, r *http.Request) {
	var req subdomain.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	id, total, err := a.subdomain.Start(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"scan_id": id, "total": total})
}

func (a *API) subdomainStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.subdomain.Stop(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (a *API) subdomainStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, total, results, warning, ok := a.subdomain.Snapshot(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"status": status, "total": total, "results": results, "warning": warning})
}
