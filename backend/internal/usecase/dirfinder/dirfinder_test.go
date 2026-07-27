package dirfinder

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"nullsec/backend/internal/adapter/ws"
	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/infra/upstream"
)

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeHTTPClientProvider satisfies HTTPClientProvider, ignoring the
// requested timeout and always returning a client wired to the given
// RoundTripper — lets tests stand in for the target without a real network
// call.
type fakeHTTPClientProvider struct {
	rt http.RoundTripper
}

func (f fakeHTTPClientProvider) HTTPClient(time.Duration) *http.Client {
	return &http.Client{Transport: f.rt}
}

func TestCandidatePaths(t *testing.T) {
	orig := Wordlist
	Wordlist = []string{"admin", "backup"}
	defer func() { Wordlist = orig }()

	got := candidatePaths(2, []string{"php", "bak"})
	want := []string{"admin", "admin.php", "admin.bak", "backup", "backup.php", "backup.bak"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestCandidatePaths_ClampsWordlistSize(t *testing.T) {
	orig := Wordlist
	Wordlist = []string{"a", "b", "c"}
	defer func() { Wordlist = orig }()

	got := candidatePaths(100, nil)
	if len(got) != 3 {
		t.Fatalf("got %d candidates, want clamped to len(Wordlist)=3", len(got))
	}
}

func TestManagerStart_Validation(t *testing.T) {
	m := NewManager(ws.NewHub(), upstream.NewManager())

	if _, _, err := m.Start(StartRequest{Target: "", WordlistSize: 1}); err == nil {
		t.Error("expected an error for an empty target")
	}
	if _, _, err := m.Start(StartRequest{Target: "example.com", WordlistSize: 0}); err == nil {
		t.Error("expected an error for a zero wordlist_size")
	}
}

func TestManagerStart_TooManyCandidates(t *testing.T) {
	orig := Wordlist
	Wordlist = make([]string, MaxCandidates+1)
	for i := range Wordlist {
		Wordlist[i] = fmt.Sprintf("word%d", i)
	}
	defer func() { Wordlist = orig }()

	m := NewManager(ws.NewHub(), upstream.NewManager())
	_, _, err := m.Start(StartRequest{Target: "example.com", WordlistSize: MaxCandidates + 1})
	if err == nil {
		t.Fatal("expected an error when candidate count exceeds MaxCandidates")
	}
}

func TestManagerRun_ReportsResultsAndRedirectsWithoutFollowing(t *testing.T) {
	orig := Wordlist
	Wordlist = []string{"admin", "missing"}
	defer func() { Wordlist = orig }()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/admin") {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": {"/admin/login"}},
				Body:       http.NoBody,
			}, nil
		}
		return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
	})

	m := NewManager(ws.NewHub(), fakeHTTPClientProvider{rt: rt})
	id, total, err := m.Start(StartRequest{Target: "https://example.com", WordlistSize: 2})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}

	deadline := time.Now().Add(2 * time.Second)
	var results []Result
	for time.Now().Before(deadline) {
		status, _, r, ok := m.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot: scan id vanished")
		}
		if status == entity.ScanDone {
			results = r
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	byPath := make(map[string]Result, len(results))
	for _, r := range results {
		byPath[r.Path] = r
	}
	if r := byPath["admin"]; r.Status != http.StatusFound || r.Redirect != "/admin/login" {
		t.Errorf("admin result = %+v, want status=302 redirect=/admin/login", r)
	}
	if r := byPath["missing"]; r.Status != http.StatusNotFound {
		t.Errorf("missing result = %+v, want status=404", r)
	}
}

func TestManagerStop(t *testing.T) {
	orig := Wordlist
	Wordlist = []string{"a", "b", "c", "d", "e"}
	defer func() { Wordlist = orig }()

	block := make(chan struct{})
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		<-block
		return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
	})

	m := NewManager(ws.NewHub(), fakeHTTPClientProvider{rt: rt})
	id, _, err := m.Start(StartRequest{Target: "example.com", WordlistSize: 5, Concurrency: 1})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !m.Stop(id) {
		t.Fatal("Stop returned false for a known scan id")
	}
	close(block)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status, _, _, ok := m.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot: scan id vanished")
		}
		if status == entity.ScanStopped || status == entity.ScanDone {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("scan never settled into stopped/done after Stop")
}

func TestManagerStop_UnknownID(t *testing.T) {
	m := NewManager(ws.NewHub(), upstream.NewManager())
	if m.Stop("does-not-exist") {
		t.Fatal("expected Stop to return false for an unknown id")
	}
}

func TestManagerSnapshot_UnknownID(t *testing.T) {
	m := NewManager(ws.NewHub(), upstream.NewManager())
	if _, _, _, ok := m.Snapshot("does-not-exist"); ok {
		t.Fatal("expected Snapshot to return ok=false for an unknown id")
	}
}
