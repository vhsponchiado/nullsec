package httpapi_test

import (
	"net/http"
	"testing"
	"time"
)

func TestSubdomain_WordlistSize(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/subdomain/wordlist-size", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var size struct {
		Size int `json:"size"`
	}
	decode(t, rec, &size)
	if size.Size == 0 {
		t.Fatal("expected a non-zero built-in subdomain wordlist size")
	}
}

func TestSubdomain_StartRequiresCRTSHOrWordlist(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/subdomain/start", map[string]any{
		"domain":        "example.com",
		"use_crtsh":     false,
		"wordlist_size": 0,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when neither crt.sh nor DNS brute-force is enabled", rec.Code)
	}
}

func TestSubdomain_StartMissingDomain(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/subdomain/start", map[string]any{
		"use_crtsh": true,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a missing domain", rec.Code)
	}
}

// TestSubdomain_StartAndStop only exercises the HTTP wiring (start reports a
// running job, stop actually stops it) — it deliberately doesn't wait for a
// real DNS brute-force to finish, since that (unlike this project's other
// scanners) always hits real DNS regardless of the upstream-proxy setting,
// which would make this test dependent on real network conditions.
func TestSubdomain_StartAndStop(t *testing.T) {
	h := newHarness(t)

	rec := h.doJSON(http.MethodPost, "/api/tools/subdomain/start", map[string]any{
		"domain":        "example.com",
		"use_crtsh":     false,
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
	// wordlist_size candidates plus the root domain itself, always checked
	// too — see Manager.Start's `wordlistSize + 1`.
	if start.ScanID == "" || start.Total != 2 {
		t.Fatalf("start response = %+v, want total=2 (root domain + 1 brute-forced candidate)", start)
	}

	statusRec := h.do(http.MethodGet, "/api/tools/subdomain/"+start.ScanID, nil)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status status = %d", statusRec.Code)
	}

	stopRec := h.do(http.MethodPost, "/api/tools/subdomain/"+start.ScanID+"/stop", nil)
	if stopRec.Code != http.StatusOK {
		t.Fatalf("stop status = %d", stopRec.Code)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec := h.do(http.MethodGet, "/api/tools/subdomain/"+start.ScanID, nil)
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

func TestSubdomain_StopUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/tools/subdomain/does-not-exist/stop", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestSubdomain_StatusUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/subdomain/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
