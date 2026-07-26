package mitm

import "net/http"

// retryOnceTransport retries idempotent, bodyless requests exactly once when
// the round trip fails outright (e.g. EOF from a server closing a reused
// keep-alive connection right as we try to send on it). Real browser traffic
// is mostly GET with no body, so this covers the common transient-failure case.
type retryOnceTransport struct {
	rt http.RoundTripper
}

func (t *retryOnceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.rt.RoundTrip(req)
	if err == nil {
		return resp, nil
	}
	if (req.Method != http.MethodGet && req.Method != http.MethodHead) || req.ContentLength != 0 {
		return resp, err
	}
	return t.rt.RoundTrip(req)
}

// NewTransport returns the http.RoundTripper the proxy engine sends every
// outbound request through — the standard library's transport plus one
// retry for a transient failure on an idempotent request.
func NewTransport() http.RoundTripper {
	return &retryOnceTransport{rt: http.DefaultTransport}
}
