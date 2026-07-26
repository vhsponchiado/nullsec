package httpapi

import (
	"encoding/json"
	"net/http"

	"nullsec/backend/internal/usecase/repeater"
)

type repeaterRequest struct {
	Method  string      `json:"method"`
	URL     string      `json:"url"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
	// InterceptResponse routes this send through the real proxy pipeline
	// with the response forced into the Intercept > Responses queue, instead
	// of the normal direct fire-and-return-the-answer path. Lets a single
	// request (e.g. "resend this POST but let me edit its response") be
	// intercepted without flipping the global toggle for all live traffic.
	InterceptResponse bool `json:"interceptResponse"`
}

func (a *API) repeaterSend(w http.ResponseWriter, r *http.Request) {
	var reqBody repeaterRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if reqBody.Method == "" || reqBody.URL == "" {
		http.Error(w, "method and url are required", http.StatusBadRequest)
		return
	}

	resp := a.repeater.Send(repeater.Request{
		Method:   reqBody.Method,
		URL:      reqBody.URL,
		Headers:  reqBody.Headers,
		Body:     reqBody.Body,
		ViaProxy: reqBody.InterceptResponse,
	})
	writeJSON(w, resp)
}
