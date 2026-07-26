package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"nullsec/backend/internal/usecase/intruder"
)

func (a *API) intruderStart(w http.ResponseWriter, r *http.Request) {
	var req intruder.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	id, total, err := a.intruder.Start(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"attack_id": id, "total": total})
}

func (a *API) intruderStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.intruder.Stop(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (a *API) intruderStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, total, results, ok := a.intruder.Snapshot(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"status": status, "total": total, "results": results})
}

func (a *API) intruderResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}
	result, ok := a.intruder.Result(id, index)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, result)
}

func (a *API) wordlistPasswords(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.intruder.Passwords())
}

func (a *API) wordlistUsernames(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.intruder.Usernames())
}

func (a *API) wordlistCredentials(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"usernames": a.intruder.CredentialUsernames(),
		"passwords": a.intruder.CredentialPasswords(),
	})
}
