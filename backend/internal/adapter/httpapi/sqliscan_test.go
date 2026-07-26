package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestSqliscan_StartNoMatches(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/sqli-scan/start", map[string]any{"host": "nothing-here.example"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when no history entries match", rec.Code)
	}
}

func TestSqliscan_StartAndComplete(t *testing.T) {
	h := newHarness(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()

	targetURL := target.URL + "/?id=1"
	host := mustHost(t, target.URL)
	seedHistory(t, h, "req-1", host, targetURL)

	rec := h.doJSON(http.MethodPost, "/api/tools/sqli-scan/start", map[string]any{"host": host})
	if rec.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", rec.Code, rec.Body.String())
	}
	var start struct {
		ScanID string `json:"scan_id"`
		Total  int    `json:"total"`
	}
	decode(t, rec, &start)
	if start.ScanID == "" || start.Total != 1 {
		t.Fatalf("start response = %+v", start)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		statusRec := h.do(http.MethodGet, "/api/tools/sqli-scan/"+start.ScanID, nil)
		var status struct {
			Status  string `json:"status"`
			Results []any  `json:"results"`
		}
		decode(t, statusRec, &status)
		if status.Status == "done" {
			if len(status.Results) != 1 {
				t.Fatalf("expected 1 result once done, got %+v", status.Results)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan never reached status=done")
}

func TestSqliscan_StopUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/tools/sqli-scan/does-not-exist/stop", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestSqliscan_StatusUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/sqli-scan/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func mustHost(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	return u.Host
}
