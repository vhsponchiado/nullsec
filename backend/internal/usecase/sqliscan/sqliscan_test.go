package sqliscan

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nullsec/backend/internal/adapter/ws"
	"nullsec/backend/internal/entity"
)

// fakeHistory is an in-memory entity.HistoryRepository stub — only List
// (filtered by Host) is exercised by sqliscan, so every other method is a
// bare-minimum implementation to satisfy the interface.
type fakeHistory struct {
	items []entity.Request
}

func (f *fakeHistory) Insert(r *entity.Request) error { f.items = append(f.items, *r); return nil }
func (f *fakeHistory) Get(id string) (*entity.Request, error) {
	for _, r := range f.items {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (f *fakeHistory) List(filter entity.RequestFilter) ([]entity.Request, error) {
	if filter.Host == "" {
		return f.items, nil
	}
	var out []entity.Request
	for _, r := range f.items {
		if r.Host == filter.Host {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeHistory) ListSummary(filter entity.RequestFilter) ([]entity.RequestSummary, error) {
	return nil, nil
}
func (f *fakeHistory) Count(filter entity.RequestFilter) (int, error) { return len(f.items), nil }
func (f *fakeHistory) Clear() error                                   { f.items = nil; return nil }

func TestDedupe_DropsRepeatedMethodURLBody(t *testing.T) {
	items := []entity.Request{
		{ID: "1", Method: "GET", URL: "http://a/?q=1", Body: ""},
		{ID: "2", Method: "GET", URL: "http://a/?q=1", Body: ""},  // exact repeat of #1
		{ID: "3", Method: "GET", URL: "http://a/?q=2", Body: ""},  // different query
		{ID: "4", Method: "POST", URL: "http://a/?q=1", Body: ""}, // different method
		{ID: "5", Method: "GET", URL: "http://a/?q=1", Body: "x"}, // different body
	}
	got := dedupe(items)
	if len(got) != 4 {
		t.Fatalf("expected 4 unique items (one exact duplicate dropped), got %d: %+v", len(got), got)
	}
	if got[0].ID != "1" {
		t.Errorf("expected the first occurrence (id=1) to be kept, got id=%s", got[0].ID)
	}
}

func TestManagerStart_NoMatchingHistory(t *testing.T) {
	m := NewManager(ws.NewHub(), &fakeHistory{})
	if _, _, err := m.Start(StartRequest{Host: "example.com"}); err == nil {
		t.Error("expected an error when no history items match the filter")
	}
}

func TestManagerStart_ScansMatchingHistoryAndStreamsResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A recognizable DB error string in the body is sqli's error-based
		// signal — deliberately triggered so the scan produces a non-empty,
		// Vulnerable finding to assert on.
		if q := r.URL.Query().Get("id"); q != "1" {
			fmt.Fprint(w, "you have an error in your sql syntax near '"+q+"'")
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	history := &fakeHistory{items: []entity.Request{
		{ID: "1", Method: "GET", URL: srv.URL + "/?id=1", Host: "target"},
	}}
	m := NewManager(ws.NewHub(), history)

	id, total, err := m.Start(StartRequest{Host: "target"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if total != 1 {
		t.Fatalf("got total=%d, want 1", total)
	}

	deadline := time.Now().Add(10 * time.Second)
	var results []ItemResult
	for time.Now().Before(deadline) {
		status, _, r, ok := m.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot: scan id vanished")
		}
		if status == entity.ScanDone {
			results = r
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 scanned item once done, got %d", len(results))
	}
	if results[0].Report.Summary.Vulnerable == 0 {
		t.Errorf("expected the error-based signal to be flagged vulnerable: %+v", results[0].Report)
	}
}
