package httpapi_test

import (
	"net/http"
	"testing"

	proxyuc "nullsec/backend/internal/usecase/proxy"
)

func TestInterceptRequest_ToggleAndStatus(t *testing.T) {
	h := newHarness(t)

	rec := h.do(http.MethodGet, "/api/intercept/status", nil)
	var status struct {
		Enabled bool `json:"enabled"`
	}
	decode(t, rec, &status)
	if status.Enabled {
		t.Fatal("intercept should start disabled")
	}

	rec = h.doJSON(http.MethodPost, "/api/intercept/toggle", map[string]bool{"enabled": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d", rec.Code)
	}
	if !h.interceptor.Enabled() {
		t.Fatal("toggle endpoint should have enabled the interceptor")
	}

	rec = h.do(http.MethodGet, "/api/intercept/status", nil)
	decode(t, rec, &status)
	if !status.Enabled {
		t.Fatal("status endpoint should reflect the toggle")
	}
}

func TestInterceptRequest_ListPending(t *testing.T) {
	h := newHarness(t)
	h.interceptor.Register(proxyuc.PendingRequest{ID: "req-1", Method: "GET", URL: "http://example.com/"})

	rec := h.do(http.MethodGet, "/api/intercept/pending", nil)
	var pending []proxyuc.PendingRequest
	decode(t, rec, &pending)
	if len(pending) != 1 || pending[0].ID != "req-1" {
		t.Fatalf("pending = %+v, want a single req-1 entry", pending)
	}
}

func TestInterceptRequest_ForwardWithEdits(t *testing.T) {
	h := newHarness(t)
	h.interceptor.Register(proxyuc.PendingRequest{
		ID:      "req-1",
		Method:  "GET",
		URL:     "http://example.com/",
		Headers: http.Header{"X-Original": {"1"}},
		Body:    "original",
	})

	rec := h.doJSON(http.MethodPost, "/api/intercept/req-1/forward", map[string]any{
		"method": "POST",
		"url":    "http://edited.example/",
		"body":   "edited",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("forward status = %d body=%s", rec.Code, rec.Body.String())
	}

	if _, ok := h.interceptor.Get("req-1"); ok {
		t.Fatal("forwarding should remove the pending entry")
	}
}

func TestInterceptRequest_ForwardFallsBackToOriginalWhenBodyOmitsFields(t *testing.T) {
	h := newHarness(t)
	h.interceptor.Register(proxyuc.PendingRequest{
		ID:     "req-1",
		Method: "GET",
		URL:    "http://example.com/",
		Body:   "original",
	})

	// An empty JSON object means "forward unchanged" — every edited.* field
	// is its zero value, so forwardIntercepted must keep the original
	// method/url/body instead of blanking them out.
	rec := h.doJSON(http.MethodPost, "/api/intercept/req-1/forward", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("forward status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestInterceptRequest_ForwardUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/intercept/does-not-exist/forward", map[string]any{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestInterceptRequest_ForwardTwice guards against double-forwarding: once
// Resolve has removed the pending entry, a second forward on the same id
// must not silently succeed (or double-deliver a decision) — it 404s the
// same way forwarding an id that was never registered does, because the
// handler's first step (Get) no longer finds it.
func TestInterceptRequest_ForwardTwice(t *testing.T) {
	h := newHarness(t)
	h.interceptor.Register(proxyuc.PendingRequest{ID: "req-1", Method: "GET", URL: "http://example.com/"})

	first := h.doJSON(http.MethodPost, "/api/intercept/req-1/forward", map[string]any{})
	if first.Code != http.StatusOK {
		t.Fatalf("first forward status = %d", first.Code)
	}

	second := h.doJSON(http.MethodPost, "/api/intercept/req-1/forward", map[string]any{})
	if second.Code != http.StatusNotFound {
		t.Fatalf("second forward status = %d, want 404 (already removed from pending)", second.Code)
	}
}

func TestInterceptRequest_Drop(t *testing.T) {
	h := newHarness(t)
	h.interceptor.Register(proxyuc.PendingRequest{ID: "req-1", Method: "GET", URL: "http://example.com/"})

	rec := h.do(http.MethodPost, "/api/intercept/req-1/drop", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("drop status = %d", rec.Code)
	}
	if _, ok := h.interceptor.Get("req-1"); ok {
		t.Fatal("drop should remove the pending entry")
	}
}

// TestInterceptRequest_DropUnknownID: unlike forward, drop doesn't Get
// before Resolve — an unknown id and an already-resolved one are
// indistinguishable to it, so both 409 rather than one 404ing.
func TestInterceptRequest_DropUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/intercept/does-not-exist/drop", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestInterceptResponse_ToggleAndStatus(t *testing.T) {
	h := newHarness(t)

	rec := h.doJSON(http.MethodPost, "/api/intercept/response/toggle", map[string]bool{"enabled": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d", rec.Code)
	}
	if !h.respIntercept.Enabled() {
		t.Fatal("toggle endpoint should have enabled the response interceptor")
	}

	rec = h.do(http.MethodGet, "/api/intercept/response/status", nil)
	var status struct {
		Enabled bool `json:"enabled"`
	}
	decode(t, rec, &status)
	if !status.Enabled {
		t.Fatal("status endpoint should reflect the toggle")
	}
}

func TestInterceptResponse_ListPending(t *testing.T) {
	h := newHarness(t)
	h.respIntercept.Register(proxyuc.PendingResponse{ID: "resp-1", Status: 200, RequestURL: "http://example.com/"})

	rec := h.do(http.MethodGet, "/api/intercept/response/pending", nil)
	var pending []proxyuc.PendingResponse
	decode(t, rec, &pending)
	if len(pending) != 1 || pending[0].ID != "resp-1" {
		t.Fatalf("pending = %+v, want a single resp-1 entry", pending)
	}
}

func TestInterceptResponse_ForwardWithEdits(t *testing.T) {
	h := newHarness(t)
	h.respIntercept.Register(proxyuc.PendingResponse{
		ID:         "resp-1",
		Status:     200,
		RequestURL: "http://example.com/",
		Headers:    http.Header{"Content-Type": {"text/plain"}},
		Body:       "original",
	})

	rec := h.doJSON(http.MethodPost, "/api/intercept/response/resp-1/forward", map[string]any{
		"status": 201,
		"body":   "edited",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("forward status = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := h.respIntercept.Get("resp-1"); ok {
		t.Fatal("forwarding should remove the pending entry")
	}
}

func TestInterceptResponse_Drop(t *testing.T) {
	h := newHarness(t)
	h.respIntercept.Register(proxyuc.PendingResponse{ID: "resp-1", Status: 200})

	rec := h.do(http.MethodPost, "/api/intercept/response/resp-1/drop", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("drop status = %d", rec.Code)
	}
	if _, ok := h.respIntercept.Get("resp-1"); ok {
		t.Fatal("drop should remove the pending entry")
	}
}

func TestInterceptResponse_DropUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/intercept/response/does-not-exist/drop", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (drop doesn't distinguish unknown from already-resolved)", rec.Code)
	}
}
