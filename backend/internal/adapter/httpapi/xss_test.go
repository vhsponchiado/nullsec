package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestXss_Payloads(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/xss-payloads", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var payloads []map[string]any
	decode(t, rec, &payloads)
	if len(payloads) == 0 {
		t.Fatal("expected a non-empty built-in XSS payload list")
	}
}

func TestXss_Check_Success(t *testing.T) {
	h := newHarness(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>" + r.URL.Query().Get("q") + "</html>"))
	}))
	defer target.Close()

	rec := h.doJSON(http.MethodPost, "/api/tools/xss-check", map[string]any{
		"url": target.URL + "/?q=hello",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var report struct {
		Summary struct {
			ParamsTested int `json:"params_tested"`
		} `json:"summary"`
	}
	decode(t, rec, &report)
	if report.Summary.ParamsTested != 1 {
		t.Fatalf("expected the single ?q= param to be tested, got %+v", report.Summary)
	}
}

func TestXss_Check_MissingURL(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/xss-check", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
