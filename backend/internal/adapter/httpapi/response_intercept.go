package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/usecase/proxy"
)

func (a *API) toggleResponseIntercept(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	a.responseInterceptor.SetEnabled(body.Enabled)
	writeJSON(w, map[string]bool{"enabled": body.Enabled})
}

func (a *API) responseInterceptStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{"enabled": a.responseInterceptor.Enabled()})
}

func (a *API) listPendingResponses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.responseInterceptor.List())
}

func (a *API) forwardInterceptedResponse(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	original, ok := a.responseInterceptor.Get(id)
	if !ok {
		http.Error(w, "no pending response with that id", http.StatusNotFound)
		return
	}

	decision := proxy.ResponseDecision{
		Action:  "forward",
		Status:  original.Status,
		Headers: original.Headers,
		Body:    original.Body,
	}

	var edited struct {
		Status  int         `json:"status"`
		Headers http.Header `json:"headers"`
		Body    string      `json:"body"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&edited); err == nil {
			if edited.Status != 0 {
				decision.Status = edited.Status
			}
			if len(edited.Headers) > 0 {
				decision.Headers = edited.Headers
			}
			if edited.Body != "" {
				decision.Body = edited.Body
			}
		}
	}

	if !a.responseInterceptor.Resolve(id, decision) {
		http.Error(w, "response already resolved", http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"status": "forwarded"})
}

func (a *API) dropInterceptedResponse(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.responseInterceptor.Resolve(id, proxy.ResponseDecision{Action: "drop"}) {
		http.Error(w, "response already resolved", http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"status": "dropped"})
}
