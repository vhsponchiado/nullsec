package httpapi_test

import (
	"net/http"
	"testing"
	"time"
)

func TestDirfinder_WordlistSize(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/dirfinder/wordlist-size", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var size struct {
		Size int `json:"size"`
	}
	decode(t, rec, &size)
	if size.Size == 0 {
		t.Fatal("expected a non-zero built-in directory wordlist size")
	}
}

func TestDirfinder_StartMissingTarget(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/dirfinder/start", map[string]any{
		"wordlist_size": 5,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a missing target", rec.Code)
	}
}

func TestDirfinder_StartMissingWordlistSize(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/dirfinder/start", map[string]any{
		"target": "example.com",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a zero wordlist_size", rec.Code)
	}
}

// TestDirfinder_StartAndStop only exercises the HTTP wiring (start reports a
// running job, stop actually stops it) — it deliberately doesn't wait for a
// real scan to finish, since this (like Subdomain Finder and Port Scanner)
// always hits the real network regardless of the upstream-proxy setting.
func TestDirfinder_StartAndStop(t *testing.T) {
	h := newHarness(t)

	rec := h.doJSON(http.MethodPost, "/api/tools/dirfinder/start", map[string]any{
		"target":        "example.com",
		"wordlist_size": 1,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", rec.Code, rec.Body.String())
	}
	var start struct {
		ScanID string `json:"scan_id"`
		Total  int    `json:"total"`
	}
	decode(t, rec, &start)
	if start.ScanID == "" || start.Total != 1 {
		t.Fatalf("start response = %+v, want total=1", start)
	}

	statusRec := h.do(http.MethodGet, "/api/tools/dirfinder/"+start.ScanID, nil)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status status = %d", statusRec.Code)
	}

	stopRec := h.do(http.MethodPost, "/api/tools/dirfinder/"+start.ScanID+"/stop", nil)
	if stopRec.Code != http.StatusOK {
		t.Fatalf("stop status = %d", stopRec.Code)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec := h.do(http.MethodGet, "/api/tools/dirfinder/"+start.ScanID, nil)
		var status struct {
			Status string `json:"status"`
		}
		decode(t, rec, &status)
		if status.Status == "stopped" || status.Status == "done" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan never settled into stopped/done after Stop")
}

func TestDirfinder_StopUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/tools/dirfinder/does-not-exist/stop", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDirfinder_StatusUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/dirfinder/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
