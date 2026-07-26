package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"nullsec/backend/internal/usecase/xss"
)

func (a *API) xssPayloads(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, xss.Payloads)
}

func (a *API) xssCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Method  string      `json:"method"`
		URL     string      `json:"url"`
		Headers http.Header `json:"headers"`
		Body    string      `json:"body"`
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

	method := strings.ToUpper(strings.TrimSpace(body.Method))
	if method == "" {
		method = http.MethodGet
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	report := xss.Run(ctx, xss.Request{
		Method:  method,
		URL:     target,
		Headers: body.Headers,
		Body:    body.Body,
	})
	writeJSON(w, report)
}
