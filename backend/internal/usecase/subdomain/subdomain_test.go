package subdomain

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"nullsec/backend/internal/adapter/ws"
	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/infra/upstream"
)

// crtshRetryDelayForTest shrinks the package-level retry delay for the
// duration of a test and returns a func restoring it, so retry tests don't
// actually wait crtshRetryDelay in real time.
func crtshRetryDelayForTest() func() {
	orig := crtshRetryDelay
	crtshRetryDelay = time.Millisecond
	return func() { crtshRetryDelay = orig }
}

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeHTTPClientProvider satisfies HTTPClientProvider, ignoring the
// requested timeout and always returning a client wired to the given
// RoundTripper — lets tests stand in for crt.sh without a real network call.
type fakeHTTPClientProvider struct {
	rt http.RoundTripper
}

func (f fakeHTTPClientProvider) HTTPClient(time.Duration) *http.Client {
	return &http.Client{Transport: f.rt}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestQueryCRTSH_RetriesOnTransientFailure(t *testing.T) {
	calls := 0
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("connection reset by peer")
		}
		return jsonResponse(`[{"name_value":"api.example.com"}]`), nil
	})

	// Avoid the test actually sleeping crtshRetryDelay between attempts.
	origDelay := crtshRetryDelayForTest()
	defer origDelay()

	names, err := queryCRTSH(context.Background(), fakeHTTPClientProvider{rt: rt}, "example.com")
	if err != nil {
		t.Fatalf("expected the retry to succeed, got error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected exactly 2 attempts (1 failure + 1 success), got %d", calls)
	}
	if len(names) != 1 || names[0] != "api.example.com" {
		t.Fatalf("got names=%v, want [api.example.com]", names)
	}
}

func TestQueryCRTSH_GivesUpAfterMaxRetries(t *testing.T) {
	calls := 0
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("connection reset by peer")
	})

	defer crtshRetryDelayForTest()()

	_, err := queryCRTSH(context.Background(), fakeHTTPClientProvider{rt: rt}, "example.com")
	if err == nil {
		t.Fatal("expected an error once every attempt fails")
	}
	if calls != crtshRetries {
		t.Fatalf("got %d attempts, want exactly crtshRetries=%d", calls, crtshRetries)
	}
}

// TestManagerRun_WarnsOnEmptyCRTSHResult exercises the full Start/Snapshot
// path with a crt.sh stand-in that succeeds (no error) but returns zero
// certificates — the case that used to silently look identical to "this
// domain has no other subdomains," rather than "crt.sh might have given us
// a truncated answer."
func TestManagerRun_WarnsOnEmptyCRTSHResult(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`[]`), nil
	})

	m := NewManager(ws.NewHub(), fakeHTTPClientProvider{rt: rt})
	id, _, err := m.Start(StartRequest{Domain: "example.com", UseCRTSH: true})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var warning string
	for time.Now().Before(deadline) {
		status, _, _, w, ok := m.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot: scan id vanished")
		}
		if status != "" && status != entity.ScanRunning {
			warning = w
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !strings.Contains(warning, "crt.sh returned no certificates") {
		t.Fatalf("expected a warning about the empty crt.sh result, got %q", warning)
	}
}

func TestNormalizeDomain(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare domain", "example.com", "example.com"},
		{"https URL", "https://example.com", "example.com"},
		{"http URL with path", "http://example.com/path", "example.com"},
		{"URL with port", "example.com:8080", "example.com"},
		{"URL with query string", "example.com?x=1", "example.com"},
		{"uppercase", "EXAMPLE.com", "example.com"},
		{"surrounding whitespace", "  example.com  ", "example.com"},
		{"full URL with everything", "HTTPS://Example.com:443/a/b?x=1#frag", "example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeDomain(tc.in); got != tc.want {
				t.Errorf("NormalizeDomain(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestManagerStart_Validation(t *testing.T) {
	m := NewManager(ws.NewHub(), upstream.NewManager())

	if _, _, err := m.Start(StartRequest{Domain: ""}); err == nil {
		t.Error("expected an error for an empty domain")
	}
	if _, _, err := m.Start(StartRequest{Domain: "example.com", UseCRTSH: false, WordlistSize: 0}); err == nil {
		t.Error("expected an error when neither crt.sh nor DNS brute-force is enabled")
	}

	id, total, err := m.Start(StartRequest{Domain: "example.com", UseCRTSH: false, WordlistSize: 5})
	if err != nil {
		t.Fatalf("unexpected error on a valid request: %v", err)
	}
	if id == "" || total != 6 { // wordlistSize + 1, per Start's doc comment
		t.Fatalf("got id=%q total=%d, want a non-empty id and total=6", id, total)
	}
	m.Stop(id)
}

func TestManagerStart_ClampsWordlistSizeToAvailableWords(t *testing.T) {
	m := NewManager(ws.NewHub(), upstream.NewManager())
	huge := len(Wordlist) + 1000

	id, total, err := m.Start(StartRequest{Domain: "example.com", WordlistSize: huge})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != len(Wordlist)+1 {
		t.Fatalf("got total=%d, want clamped to len(Wordlist)+1=%d", total, len(Wordlist)+1)
	}
	m.Stop(id)
}
