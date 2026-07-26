package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"nullsec/backend/internal/entity"
)

func TestPassiveScan_FindsMissingSecurityHeaders(t *testing.T) {
	h := newHarness(t)
	if err := h.history.Insert(&entity.Request{
		ID:          "req-1",
		Method:      "GET",
		URL:         "http://example.com/",
		Host:        "example.com",
		Headers:     "{}",
		Status:      200,
		RespHeaders: "{}", // no security headers at all
		RespBody:    "hello",
		Outcome:     "completed",
		CreatedAt:   time.Now(),
	}); err != nil {
		t.Fatalf("seed history: %v", err)
	}

	rec := h.do(http.MethodGet, "/api/tools/passive-scan", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var report struct {
		Findings []map[string]any `json:"findings"`
		Summary  struct {
			RequestsScanned int `json:"requests_scanned"`
			Findings        int `json:"findings"`
		} `json:"summary"`
	}
	decode(t, rec, &report)
	if report.Summary.RequestsScanned != 1 {
		t.Fatalf("summary = %+v, want requests_scanned=1", report.Summary)
	}
	if report.Summary.Findings == 0 || len(report.Findings) == 0 {
		t.Fatal("expected the missing security headers to be flagged")
	}
}

func TestPassiveScan_EmptyHistory(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/passive-scan", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var report struct {
		Summary struct {
			RequestsScanned int `json:"requests_scanned"`
		} `json:"summary"`
	}
	decode(t, rec, &report)
	if report.Summary.RequestsScanned != 0 {
		t.Fatalf("requests_scanned = %d, want 0 for an empty history", report.Summary.RequestsScanned)
	}
}

func TestPassiveScan_HostFilter(t *testing.T) {
	h := newHarness(t)
	base := entity.Request{Headers: "{}", RespHeaders: "{}", Status: 200, Outcome: "completed", CreatedAt: time.Now()}

	a := base
	a.ID, a.Host, a.URL, a.Method = "req-a", "a.example", "http://a.example/", "GET"
	b := base
	b.ID, b.Host, b.URL, b.Method = "req-b", "b.example", "http://b.example/", "GET"
	if err := h.history.Insert(&a); err != nil {
		t.Fatal(err)
	}
	if err := h.history.Insert(&b); err != nil {
		t.Fatal(err)
	}

	rec := h.do(http.MethodGet, "/api/tools/passive-scan?host=a.example", nil)
	var report struct {
		Summary struct {
			RequestsScanned int `json:"requests_scanned"`
		} `json:"summary"`
	}
	decode(t, rec, &report)
	if report.Summary.RequestsScanned != 1 {
		t.Fatalf("host filter should narrow to 1 request, got %+v", report.Summary)
	}
}
