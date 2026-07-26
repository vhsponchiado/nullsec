package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestXssscan_StartNoMatches(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/xss-scan/start", map[string]any{"host": "nothing-here.example"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when no history entries match", rec.Code)
	}
}

func TestXssscan_StartAndComplete(t *testing.T) {
	h := newHarness(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>" + r.URL.Query().Get("q") + "</html>"))
	}))
	defer target.Close()

	targetURL := target.URL + "/?q=hi"
	host := mustHost(t, target.URL)
	seedHistory(t, h, "req-1", host, targetURL)

	rec := h.doJSON(http.MethodPost, "/api/tools/xss-scan/start", map[string]any{"host": host})
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
		statusRec := h.do(http.MethodGet, "/api/tools/xss-scan/"+start.ScanID, nil)
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

func TestXssscan_StopUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/tools/xss-scan/does-not-exist/stop", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestXssscan_StatusUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/xss-scan/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
