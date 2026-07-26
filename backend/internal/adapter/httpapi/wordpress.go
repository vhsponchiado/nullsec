package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"nullsec/backend/internal/usecase/wpcheck"
)

func (a *API) wordpressCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	target := strings.TrimSpace(body.URL)
	if target == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	results, summary := wpcheck.Run(ctx, target)
	writeJSON(w, map[string]any{"results": results, "summary": summary})
}
