package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWordpressCheck_Success(t *testing.T) {
	h := newHarness(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer target.Close()

	rec := h.doJSON(http.MethodPost, "/api/tools/wordpress-check", map[string]any{"url": target.URL})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Results []map[string]any `json:"results"`
		Summary struct {
			Checked int `json:"checked"`
		} `json:"summary"`
	}
	decode(t, rec, &body)
	if body.Summary.Checked == 0 || len(body.Results) == 0 {
		t.Fatalf("expected wpcheck to have probed at least one path, got %+v", body)
	}
}

func TestWordpressCheck_MissingURL(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/wordpress-check", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
