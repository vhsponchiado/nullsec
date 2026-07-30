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

// TestQueryCRTSHKeyword_MatchesUnrelatedApexDomains is the whole point of
// the broad search: queryCRTSH (suffix-filtered) would drop a result like
// "qa-example.com" since it doesn't end in ".example.com" — it isn't a
// subdomain, just a similarly-named, distinct domain. queryCRTSHKeyword
// must keep it.
func TestQueryCRTSHKeyword_MatchesUnrelatedApexDomains(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.RawQuery, "q=%example%") {
			t.Fatalf("expected a wildcard-both-sides query, got %q", r.URL.RawQuery)
		}
		return jsonResponse(`[{"name_value":"qa-example.com"},{"name_value":"example-staging.net"}]`), nil
	})

	names, err := queryCRTSHKeyword(context.Background(), fakeHTTPClientProvider{rt: rt}, "example")
	if err != nil {
		t.Fatalf("queryCRTSHKeyword: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("got names=%v, want both unrelated apex domains kept", names)
	}
}

// TestQueryCRTSH_DropsUnrelatedApexDomains documents the contrasting,
// existing behavior of the suffix-filtered search — it exists so a future
// change to the shared keep-predicate plumbing can't silently make
// queryCRTSH as promiscuous as queryCRTSHKeyword without a test noticing.
func TestQueryCRTSH_DropsUnrelatedApexDomains(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`[{"name_value":"qa-example.com"},{"name_value":"api.example.com"}]`), nil
	})

	names, err := queryCRTSH(context.Background(), fakeHTTPClientProvider{rt: rt}, "example.com")
	if err != nil {
		t.Fatalf("queryCRTSH: %v", err)
	}
	if len(names) != 1 || names[0] != "api.example.com" {
		t.Fatalf("got names=%v, want only the true subdomain api.example.com", names)
	}
}

func TestManagerStart_ValidationAllowsKeywordOnlyRequest(t *testing.T) {
	m := NewManager(ws.NewHub(), upstream.NewManager())

	if _, _, err := m.Start(StartRequest{Domain: "example.com", CRTSHKeyword: "  "}); err == nil {
		t.Error("expected an error when the keyword is only whitespace and nothing else is enabled")
	}

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`[]`), nil
	})
	km := NewManager(ws.NewHub(), fakeHTTPClientProvider{rt: rt})
	id, _, err := km.Start(StartRequest{Domain: "example.com", CRTSHKeyword: "example"})
	if err != nil {
		t.Fatalf("expected a keyword-only request to be valid, got: %v", err)
	}
	km.Stop(id)
}

// TestManagerRun_TagsKeywordHitsAndFlagsSameRoot exercises the full
// Start/Snapshot path with a keyword search that returns an unrelated apex
// domain, confirming it's tagged with the keyword source and SameRoot=false
// (it's not actually a subdomain of the target).
func TestManagerStart_ValidationAllowsSiblingOnlyRequest(t *testing.T) {
	m := NewManager(ws.NewHub(), upstream.NewManager())

	if _, _, err := m.Start(StartRequest{Domain: "example.com", SiblingWordlistSize: 0}); err == nil {
		t.Error("expected an error when sibling_wordlist_size is 0 and nothing else is enabled")
	}

	id, total, err := m.Start(StartRequest{Domain: "example.com", SiblingWordlistSize: 5})
	if err != nil {
		t.Fatalf("expected a sibling-only request to be valid, got: %v", err)
	}
	if total != 6 { // root + 5 sibling candidates
		t.Fatalf("got total=%d, want 6 (root + 5 sibling candidates)", total)
	}
	// Wait for the scan to actually finish — Stop only cancels the context,
	// it doesn't block until the goroutine exits, and that goroutine reads
	// the package-level Wordlist var. A later test replacing Wordlist while
	// this one's still mid-loop is exactly the kind of race that produced
	// an index-out-of-range panic before this wait was added.
	waitForTerminalStatus(t, m, id)
	m.Stop(id)
}

// waitForTerminalStatus polls Snapshot until the scan leaves ScanRunning,
// failing the test if it doesn't within 5s. Tests that mutate the
// package-level Wordlist var must call this first, since Start's goroutine
// keeps reading Wordlist until its candidate-building loops finish —
// Stop() alone only cancels the context, it doesn't wait for that.
func waitForTerminalStatus(t *testing.T, m *Manager, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, _, _, _, ok := m.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot: scan id vanished")
		}
		if status != "" && status != entity.ScanRunning {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan never reached a terminal status")
}

// TestManagerRun_SiblingBruteforceGuessesHyphenatedApexDomains is the whole
// point of this mode: DNS brute-force normally only ever tries
// prefix+"."+domain (a true subdomain). Nothing about that can ever produce
// prefix+"-"+domain (an unrelated apex domain a company registered under a
// similar name) — this active mode fills that gap for cases where the
// sibling domain was never issued a publicly-trusted cert (so crt.sh, no
// matter how it's queried, could never have found it either).
//
// This deliberately doesn't replace the package-level Wordlist var (unlike
// e.g. TestCandidatePaths-style tests elsewhere): Start spawns a goroutine
// that reads it, Stop doesn't block until that goroutine actually exits,
// and other tests in this file spawn their own Start goroutines too —
// swapping the shared slice out from under a still-running goroutine
// produced a real index-out-of-range panic during development. Reading real
// entries from the real Wordlist by index is race-free instead.
func TestManagerRun_SiblingBruteforceGuessesHyphenatedApexDomains(t *testing.T) {
	m := NewManager(ws.NewHub(), upstream.NewManager())
	id, total, err := m.Start(StartRequest{Domain: "example.com", SiblingWordlistSize: 2})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if total != 3 { // root + 2 sibling candidates
		t.Fatalf("got total=%d, want 3", total)
	}

	deadline := time.Now().Add(5 * time.Second)
	var results []Result
	for time.Now().Before(deadline) {
		status, _, r, _, ok := m.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot: scan id vanished")
		}
		if status != "" && status != entity.ScanRunning {
			results = r
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	byName := make(map[string]Result, len(results))
	for _, r := range results {
		byName[r.Subdomain] = r
	}

	for _, prefix := range Wordlist[:2] {
		want := prefix + "-example.com"
		r, ok := byName[want]
		if !ok {
			t.Fatalf("expected a candidate for %q, got results=%+v", want, results)
		}
		if r.SameRoot {
			t.Errorf("%q: expected SameRoot=false, it's a sibling apex domain, not a subdomain", want)
		}
		hasSiblingSource := false
		for _, s := range r.Sources {
			if s == sourceSiblingBrute {
				hasSiblingSource = true
			}
		}
		if !hasSiblingSource {
			t.Errorf("%q: expected sources to include %q, got %v", want, sourceSiblingBrute, r.Sources)
		}

		// A true subdomain (prefix+"."+domain) must never appear from this mode.
		if _, ok := byName[prefix+".example.com"]; ok {
			t.Errorf("sibling brute-force must never produce dot-joined subdomain candidate %q", prefix+".example.com")
		}
	}
}

func TestQueryHackerTarget_ParsesCSVAndFiltersToSameRoot(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.String(), "hackertarget.com/hostsearch") {
			t.Fatalf("expected a hackertarget hostsearch request, got %s", r.URL.String())
		}
		body := "example.com,1.2.3.4\nauth.example.com,5.6.7.8\nunrelated-example.com,9.9.9.9\n"
		return jsonResponse(body), nil
	})

	names, err := queryHackerTarget(context.Background(), fakeHTTPClientProvider{rt: rt}, "example.com")
	if err != nil {
		t.Fatalf("queryHackerTarget: %v", err)
	}
	// unrelated-example.com must be dropped: HackerTarget's hostsearch is
	// meant to be subdomain-scoped, unlike the keyword search's broad match.
	want := []string{"example.com", "auth.example.com"}
	if len(names) != len(want) {
		t.Fatalf("got names=%v, want %v", names, want)
	}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q", i, names[i], w)
		}
	}
}

func TestQueryHackerTarget_DetectsRateLimitMessage(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse("error check your search parameter. API count exceeded - Increase your API count by purchasing an API key"), nil
	})

	_, err := queryHackerTarget(context.Background(), fakeHTTPClientProvider{rt: rt}, "example.com")
	if err == nil {
		t.Fatal("expected an error when HackerTarget's response indicates the free quota is used up")
	}
}

func TestManagerStart_ValidationAllowsHackerTargetOnlyRequest(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(""), nil
	})
	m := NewManager(ws.NewHub(), fakeHTTPClientProvider{rt: rt})

	id, _, err := m.Start(StartRequest{Domain: "example.com", UseHackerTarget: true})
	if err != nil {
		t.Fatalf("expected a HackerTarget-only request to be valid, got: %v", err)
	}
	waitForTerminalStatus(t, m, id)
	m.Stop(id)
}

// TestManagerRun_TagsHackerTargetHits exercises the full Start/Snapshot path
// with a HackerTarget stand-in that finds a real subdomain, confirming it's
// tagged with the hackertarget source (and, being a true subdomain,
// SameRoot=true — unlike the keyword search's sibling-domain hits).
func TestManagerRun_TagsHackerTargetHits(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse("auth.example.com,108.139.29.74\n"), nil
	})

	m := NewManager(ws.NewHub(), fakeHTTPClientProvider{rt: rt})
	id, _, err := m.Start(StartRequest{Domain: "example.com", UseHackerTarget: true})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var results []Result
	for time.Now().Before(deadline) {
		status, _, r, _, ok := m.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot: scan id vanished")
		}
		if status != "" && status != entity.ScanRunning {
			results = r
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	var found *Result
	for i := range results {
		if results[i].Subdomain == "auth.example.com" {
			found = &results[i]
		}
	}
	if found == nil {
		t.Fatalf("expected auth.example.com in results, got %+v", results)
	}
	if !found.SameRoot {
		t.Error("expected SameRoot=true, auth.example.com is a true subdomain of example.com")
	}
	hasHackerTargetSource := false
	for _, s := range found.Sources {
		if s == sourceHackerTarget {
			hasHackerTargetSource = true
		}
	}
	if !hasHackerTargetSource {
		t.Errorf("expected sources to include %q, got %v", sourceHackerTarget, found.Sources)
	}
}

func TestManagerRun_TagsKeywordHitsAndFlagsSameRoot(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`[{"name_value":"qa-example.com"}]`), nil
	})

	m := NewManager(ws.NewHub(), fakeHTTPClientProvider{rt: rt})
	id, _, err := m.Start(StartRequest{Domain: "example.com", CRTSHKeyword: "example"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var results []Result
	for time.Now().Before(deadline) {
		status, _, r, _, ok := m.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot: scan id vanished")
		}
		if status != "" && status != entity.ScanRunning {
			results = r
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	var found *Result
	for i := range results {
		if results[i].Subdomain == "qa-example.com" {
			found = &results[i]
		}
	}
	if found == nil {
		t.Fatalf("expected qa-example.com in results, got %+v", results)
	}
	if found.SameRoot {
		t.Error("expected SameRoot=false for a keyword hit that isn't a subdomain of the target")
	}
	hasKeywordSource := false
	for _, s := range found.Sources {
		if s == sourceCRTSHKeyword {
			hasKeywordSource = true
		}
	}
	if !hasKeywordSource {
		t.Errorf("expected sources to include %q, got %v", sourceCRTSHKeyword, found.Sources)
	}
}
