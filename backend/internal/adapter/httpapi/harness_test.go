// Test harness shared by every *_test.go in this package. It wires a real
// httpapi.Handler with real, but hermetic and fast, use cases underneath —
// a temp-file SQLite history repo, a freshly generated CA, an unconfigured
// (direct-dial) upstream manager — plus a fake for the one thing a test
// suite must never do for real: making real network calls. That keeps
// these as true adapter/HTTP-boundary tests (real routing, real JSON
// encode/decode, real use-case wiring) without depending on the network.
package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"nullsec/backend/internal/adapter/httpapi"
	"nullsec/backend/internal/adapter/sqliterepo"
	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/infra/ca"
	"nullsec/backend/internal/infra/upstream"
	"nullsec/backend/internal/infra/wordlist"
	"nullsec/backend/internal/usecase/certificate"
	"nullsec/backend/internal/usecase/intruder"
	"nullsec/backend/internal/usecase/portscan"
	proxyuc "nullsec/backend/internal/usecase/proxy"
	"nullsec/backend/internal/usecase/repeater"
	"nullsec/backend/internal/usecase/settings"
	"nullsec/backend/internal/usecase/spider"
	"nullsec/backend/internal/usecase/sqliscan"
	"nullsec/backend/internal/usecase/subdomain"
	"nullsec/backend/internal/usecase/xssscan"
)

type broadcastCall struct {
	eventType string
	payload   any
}

// fakeBroadcaster is an in-memory entity.Broadcaster that records every
// call for assertions.
type fakeBroadcaster struct {
	mu    sync.Mutex
	calls []broadcastCall
}

func (f *fakeBroadcaster) Broadcast(eventType string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, broadcastCall{eventType, payload})
}

// roundTripFunc adapts a plain function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// swappableTransport wraps the Forwarder/Repeater's outbound round trip
// behind a mutex-guarded indirection — not swapped by any test today, but
// keeping the transport itself thread-safe-by-construction here means a
// future test can point it at a different stub without rebuilding the
// whole harness or auditing for data races.
type swappableTransport struct {
	mu sync.Mutex
	rt http.RoundTripper
}

func (s *swappableTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	rt := s.rt
	s.mu.Unlock()
	return rt.RoundTrip(r)
}

func okTransport() http.RoundTripper {
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})
}

type harness struct {
	t             *testing.T
	handler       http.Handler
	history       entity.HistoryRepository
	interceptor   *proxyuc.Interceptor
	respIntercept *proxyuc.ResponseInterceptor
	broadcaster   *fakeBroadcaster
	transport     *swappableTransport
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()

	history, err := sqliterepo.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open sqlite history: %v", err)
	}
	t.Cleanup(func() { history.Close() })

	certAuthority, err := ca.LoadOrCreate(filepath.Join(dir, "ca-cert.pem"), filepath.Join(dir, "ca-key.pem"))
	if err != nil {
		t.Fatalf("generate CA: %v", err)
	}

	hub := &fakeBroadcaster{}
	interceptor := proxyuc.NewInterceptor()
	responseInterceptor := proxyuc.NewResponseInterceptor()
	upstreamMgr := upstream.NewManager()
	transport := &swappableTransport{rt: okTransport()}

	forwarder := proxyuc.NewForwarder(history, hub, interceptor, responseInterceptor, transport)

	intruderMgr := intruder.NewManager(hub, upstreamMgr, wordlist.Adapter{})
	portscanMgr := portscan.NewManager(hub, upstreamMgr)
	subdomainMgr := subdomain.NewManager(hub, upstreamMgr)
	spiderMgr := spider.NewManager(hub, history, upstreamMgr)
	certificateUC := certificate.New(certAuthority)
	settingsUC := settings.New(upstreamMgr)
	repeaterUC := repeater.New(upstreamMgr, forwarder)
	xssscanMgr := xssscan.NewManager(hub, history)
	sqliscanMgr := sqliscan.NewManager(hub, history)

	handler := httpapi.NewHandler(
		history, interceptor, responseInterceptor, certificateUC,
		intruderMgr, portscanMgr, subdomainMgr, spiderMgr,
		settingsUC, repeaterUC, xssscanMgr, sqliscanMgr,
	)

	return &harness{
		t:             t,
		handler:       handler,
		history:       history,
		interceptor:   interceptor,
		respIntercept: responseInterceptor,
		broadcaster:   hub,
		transport:     transport,
	}
}

// do issues an HTTP request straight into the handler (no real listener)
// and returns the recorded response.
func (h *harness) do(method, target string, body io.Reader) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, target, body)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *harness) doJSON(method, target string, body any) *httptest.ResponseRecorder {
	h.t.Helper()
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal request body: %v", err)
		}
		r = strings.NewReader(string(data))
	}
	return h.do(method, target, r)
}

// decode JSON-decodes rec's body into v, failing the test on any error.
func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
}
