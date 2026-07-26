// Package repeater is the "resend an arbitrary request" use case behind
// the Repeater tool: either fire it directly through an upstream-aware
// HTTP client, or route it through the same intercept/history pipeline
// live proxy traffic uses (forcing the response into the Intercept >
// Responses queue) via ProxyForwarder.
package repeater

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// HTTPClientProvider hands back an *http.Client routed through whatever
// upstream proxy is currently configured. The only implementation is
// infra/upstream.Manager, but this use case doesn't know that.
type HTTPClientProvider interface {
	HTTPClient(timeout time.Duration) *http.Client
}

// ProxyForwarder routes a request through the real proxy pipeline — history
// recording, WebSocket broadcasts, and (optionally) forcing the response
// into the Intercept > Responses queue. The only implementation is
// usecase/proxy.Forwarder, but this use case doesn't know that.
type ProxyForwarder interface {
	Send(r *http.Request, forceResponseIntercept bool) (status int, headers http.Header, body []byte, dropped, timedOut bool, err error)
}

const directSendTimeout = 30 * time.Second

type Request struct {
	Method  string
	URL     string
	Headers http.Header
	Body    string
	// ViaProxy routes this send through Forwarder instead of the direct
	// HTTPClientProvider path — see ProxyForwarder's doc comment.
	ViaProxy bool
}

type Response struct {
	Status     int    `json:"status"`
	Headers    string `json:"headers"`
	Body       string `json:"body"`
	Size       int    `json:"size"`
	DurationMs int64  `json:"duration_ms"`
	Dropped    bool   `json:"dropped,omitempty"`
	TimedOut   bool   `json:"timed_out,omitempty"`
	Error      string `json:"error,omitempty"`
}

type UseCase struct {
	httpClients HTTPClientProvider
	forwarder   ProxyForwarder
}

func New(httpClients HTTPClientProvider, forwarder ProxyForwarder) *UseCase {
	return &UseCase{httpClients: httpClients, forwarder: forwarder}
}

// Send fires req.Method/URL/Headers/Body and returns the raw response,
// through whichever path req.ViaProxy selects.
func (u *UseCase) Send(req Request) Response {
	if req.ViaProxy {
		return u.sendViaProxy(req)
	}
	return u.sendDirect(req)
}

func (u *UseCase) sendDirect(req Request) Response {
	started := time.Now()

	outReq, err := http.NewRequest(req.Method, req.URL, bytes.NewReader([]byte(req.Body)))
	if err != nil {
		return Response{Error: err.Error()}
	}
	if req.Headers != nil {
		outReq.Header = req.Headers
	}

	resp, err := u.httpClients.HTTPClient(directSendTimeout).Do(outReq)
	if err != nil {
		return Response{Error: err.Error(), DurationMs: time.Since(started).Milliseconds()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{Error: err.Error(), DurationMs: time.Since(started).Milliseconds()}
	}

	headersJSON, _ := json.Marshal(resp.Header)

	return Response{
		Status:     resp.StatusCode,
		Headers:    string(headersJSON),
		Body:       string(respBody),
		Size:       len(respBody),
		DurationMs: time.Since(started).Milliseconds(),
	}
}

// sendViaProxy sends through the same pipeline real proxy traffic uses,
// forcing this one response to be held for the Intercept > Responses UI.
// The caller should switch to that tab — the held response shows up there
// via the usual response_intercepted WebSocket event.
func (u *UseCase) sendViaProxy(req Request) Response {
	started := time.Now()

	outReq, err := http.NewRequest(req.Method, req.URL, bytes.NewReader([]byte(req.Body)))
	if err != nil {
		return Response{Error: err.Error()}
	}
	if req.Headers != nil {
		outReq.Header = req.Headers
	}

	status, headers, body, dropped, timedOut, err := u.forwarder.Send(outReq, true)
	durationMs := time.Since(started).Milliseconds()

	if err != nil {
		return Response{Error: err.Error(), DurationMs: durationMs}
	}
	if dropped || timedOut {
		return Response{Dropped: dropped, TimedOut: timedOut, DurationMs: durationMs}
	}

	headersJSON, _ := json.Marshal(headers)
	return Response{
		Status:     status,
		Headers:    string(headersJSON),
		Body:       string(body),
		Size:       len(body),
		DurationMs: durationMs,
	}
}
