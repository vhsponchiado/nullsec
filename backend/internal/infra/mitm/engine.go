// Package mitm is the actual HTTP/HTTPS transport engine: it forwards plain
// HTTP requests directly (engine.go) and decrypts/re-frames HTTPS traffic
// over a CONNECT tunnel using a per-host certificate from infra/ca
// (connect.go). For both, the real business logic — the intercept
// pipeline, history recording, WebSocket broadcasts — lives one layer in,
// in usecase/proxy; this package only knows how to terminate TLS, hijack
// connections, and hand a decrypted *http.Request to that use case.
package mitm

import (
	"net/http"

	"nullsec/backend/internal/infra/ca"
	proxyuc "nullsec/backend/internal/usecase/proxy"
)

type Engine struct {
	ca        *ca.CA
	forwarder *proxyuc.Forwarder
}

func NewEngine(certAuthority *ca.CA, forwarder *proxyuc.Forwarder) *Engine {
	return &Engine{ca: certAuthority, forwarder: forwarder}
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		e.handleConnect(w, r)
		return
	}

	if !r.URL.IsAbs() {
		http.Error(w, "proxy requires absolute URI (configure your client to use this as an HTTP proxy)", http.StatusBadRequest)
		return
	}

	result := e.forwarder.Forward(r, false)

	switch {
	case result.Dropped:
		http.Error(w, "request dropped by proxy", http.StatusBadGateway)
	case result.TimedOut:
		http.Error(w, "request timed out waiting for intercept decision", http.StatusGatewayTimeout)
	case result.Err != nil:
		http.Error(w, "proxy error: "+result.Err.Error(), http.StatusBadGateway)
	default:
		for k, vv := range result.Headers {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(result.Status)
		// This is the real destination server's response body, relayed
		// verbatim to the same browser that requested it — exactly what it
		// would have received with no proxy in the middle at all. There's no
		// XSS crossing a trust boundary here for gosec's taint check to be
		// warning about.
		_, _ = w.Write(result.Body) //nolint:gosec // G705: intentional, see comment above
	}
}
