// Package proxy is the "forward a proxied request" use case: it runs a
// request through the optional intercept pipeline (pause/edit/drop/forward,
// intercept.go), performs the actual outbound round trip via an injected
// http.RoundTripper, optionally pauses the response too
// (response_intercept.go), and records the outcome via entity.HistoryRepository
// + entity.Broadcaster. It knows nothing about TLS, CONNECT tunnels, or
// hijacking raw connections — that's infra/mitm, which calls into Forward
// for both the plain-HTTP and decrypted-HTTPS paths so intercept, history,
// and Repeater all work identically for both.
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"

	"nullsec/backend/internal/entity"
)

type Forwarder struct {
	history             entity.HistoryRepository
	hub                 entity.Broadcaster
	interceptor         *Interceptor
	responseInterceptor *ResponseInterceptor
	transport           http.RoundTripper
}

func NewForwarder(history entity.HistoryRepository, hub entity.Broadcaster, interceptor *Interceptor, responseInterceptor *ResponseInterceptor, transport http.RoundTripper) *Forwarder {
	return &Forwarder{
		history:             history,
		hub:                 hub,
		interceptor:         interceptor,
		responseInterceptor: responseInterceptor,
		transport:           transport,
	}
}

// Send lets other packages (the Repeater API) route a request through the
// exact same pipeline the real proxy uses for live traffic — history
// recording, WebSocket broadcasts, and (optionally) forcing this one
// response to be held for review in the Intercept > Responses UI regardless
// of whether the global response-intercept toggle is currently on.
func (f *Forwarder) Send(r *http.Request, forceResponseIntercept bool) (status int, headers http.Header, body []byte, dropped, timedOut bool, err error) {
	result := f.Forward(r, forceResponseIntercept)
	return result.Status, result.Headers, result.Body, result.Dropped, result.TimedOut, result.Err
}

// ForwardResult carries the outcome of Forward back to whichever transport
// (a plain http.ResponseWriter, or a hijacked/TLS connection for HTTPS) needs
// to relay it to the client.
type ForwardResult struct {
	Status   int
	Headers  http.Header
	Body     []byte
	Dropped  bool
	TimedOut bool
	Err      error
}

// Forward runs a single proxied request through the intercept pipeline (if
// enabled), performs the actual outbound round trip, and records the result
// in the history repository + broadcasts. It's shared by the plain HTTP
// path, the decrypted HTTPS path (infra/mitm's tunnel loop), and Send (used
// by the Repeater API to force-intercept one specific response without
// flipping the global toggle for all live traffic).
func (f *Forwarder) Forward(r *http.Request, forceResponseIntercept bool) *ForwardResult {
	id := uuid.NewString()
	started := time.Now()

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return &ForwardResult{Err: err}
	}
	r.Body.Close()

	method := r.Method
	targetURL := r.URL.String()
	headers := cloneHeader(r.Header)
	body := rawBody

	if f.interceptor.Enabled() {
		pending := PendingRequest{
			ID:        id,
			Method:    method,
			URL:       targetURL,
			Headers:   headers,
			Body:      string(body),
			CreatedAt: started,
		}
		ch := f.interceptor.Register(pending)
		f.hub.Broadcast(entity.EventRequestIntercepted, pending)

		select {
		case decision := <-ch:
			if decision.Action == "drop" {
				f.hub.Broadcast(entity.EventRequestDropped, map[string]string{"id": id})
				f.saveRecord(id, method, targetURL, r.URL.Host, headers, body, 0, nil, nil, "dropped", started)
				return &ForwardResult{Dropped: true}
			}
			method = decision.Method
			targetURL = decision.URL
			headers = decision.Headers
			body = []byte(decision.Body)
			f.hub.Broadcast(entity.EventRequestForwarded, map[string]string{"id": id})

		case <-time.After(interceptTimeout):
			f.interceptor.Cleanup(id)
			f.hub.Broadcast(entity.EventRequestDropped, map[string]string{"id": id, "reason": "timeout"})
			// A request that times out waiting for a decision is, from the
			// history's point of view, indistinguishable from one the user
			// explicitly dropped — record it the same way, or it silently
			// vanishes instead of showing up as "dropped".
			f.saveRecord(id, method, targetURL, r.URL.Host, headers, body, 0, nil, nil, "dropped", started)
			return &ForwardResult{TimedOut: true}
		}
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return &ForwardResult{Err: err}
	}

	// targetURL is exactly what the proxy's client asked for — same as
	// destConn's target in infra/mitm's passthroughTunnel. Forwarding it
	// verbatim is the proxy's job, not an SSRF crossing a privilege boundary.
	outReq, err := http.NewRequest(method, targetURL, bytes.NewReader(body)) //nolint:gosec // G704: intentional, see comment above
	if err != nil {
		return &ForwardResult{Err: err}
	}
	outReq.Header = headers
	outReq.ContentLength = int64(len(body))
	outReq.Host = parsedURL.Host
	// The client may ask for gzip/br/zstd; we have no way to decompress br/zstd
	// (no stdlib support) and storing compressed bytes as text corrupts the
	// history/body view. Force uncompressed responses instead.
	outReq.Header.Set("Accept-Encoding", "identity")

	resp, err := f.transport.RoundTrip(outReq)
	if err != nil {
		log.Printf("proxy error for %q: %v", targetURL, err) //nolint:gosec // G706: %q quotes/escapes targetURL
		f.saveRecord(id, method, targetURL, parsedURL.Host, headers, body, 0, nil, nil, "completed", started)
		return &ForwardResult{Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ForwardResult{Err: err}
	}

	respStatus := resp.StatusCode
	respHeaders := resp.Header

	if f.responseInterceptor.Enabled() || forceResponseIntercept {
		pendingResp := PendingResponse{
			ID:            id,
			RequestMethod: method,
			RequestURL:    targetURL,
			Status:        respStatus,
			Headers:       cloneHeader(respHeaders),
			Body:          string(respBody),
		}
		ch := f.responseInterceptor.Register(pendingResp)
		f.hub.Broadcast(entity.EventResponseIntercepted, pendingResp)

		select {
		case decision := <-ch:
			if decision.Action == "drop" {
				f.hub.Broadcast(entity.EventResponseDropped, map[string]string{"id": id})
				f.saveRecord(id, method, targetURL, parsedURL.Host, headers, body, 0, nil, nil, "dropped", started)
				return &ForwardResult{Dropped: true}
			}
			respStatus = decision.Status
			respHeaders = decision.Headers
			if respHeaders == nil {
				// A caller (Forward is also reachable directly from the
				// Repeater API, not just this package's own httpapi
				// handler) may hand back a decision with no Headers set at
				// all — don't panic on the Set below over a nil map.
				respHeaders = http.Header{}
			}
			respBody = []byte(decision.Body)
			// The declared Content-Length travels with the pending response's
			// original headers, so editing the body without touching headers
			// leaves a stale value; the client then truncates/hangs waiting
			// for however many bytes the (now wrong) header promised.
			respHeaders.Set("Content-Length", strconv.Itoa(len(respBody)))
			f.hub.Broadcast(entity.EventResponseForwarded, map[string]string{"id": id})

		case <-time.After(interceptTimeout):
			f.responseInterceptor.Cleanup(id)
			f.hub.Broadcast(entity.EventResponseDropped, map[string]string{"id": id, "reason": "timeout"})
			f.saveRecord(id, method, targetURL, parsedURL.Host, headers, body, 0, nil, nil, "dropped", started)
			return &ForwardResult{TimedOut: true}
		}
	}

	f.saveRecord(id, method, targetURL, parsedURL.Host, headers, body, respStatus, respHeaders, respBody, "completed", started)

	return &ForwardResult{Status: respStatus, Headers: respHeaders, Body: respBody}
}

func (f *Forwarder) saveRecord(id, method, targetURL, host string, headers http.Header, body []byte, status int, respHeaders http.Header, respBody []byte, outcome string, started time.Time) {
	rec := &entity.Request{
		ID:          id,
		Method:      method,
		URL:         targetURL,
		Host:        host,
		Headers:     headersToJSON(headers),
		Body:        string(body),
		Status:      status,
		RespHeaders: headersToJSON(respHeaders),
		RespBody:    string(respBody),
		Size:        len(respBody),
		DurationMs:  time.Since(started).Milliseconds(),
		Outcome:     outcome,
		CreatedAt:   started,
	}

	if err := f.history.Insert(rec); err != nil {
		log.Printf("failed to save request: %v", err)
		return
	}

	if outcome == "completed" {
		// Broadcast only the summary, not rec itself — rec carries the full
		// captured headers/body/resp_headers/resp_body, which would otherwise
		// get pushed over the WS (and JSON-parsed on the main thread) for
		// every single proxied request, live traffic included.
		f.hub.Broadcast(entity.EventRequestCompleted, rec.Summary())
	}
}

func cloneHeader(hdr http.Header) http.Header {
	clone := make(http.Header, len(hdr))
	for k, v := range hdr {
		clone[k] = append([]string(nil), v...)
	}
	return clone
}

func headersToJSON(hdr http.Header) string {
	if hdr == nil {
		return "{}"
	}
	data, err := json.Marshal(hdr)
	if err != nil {
		return "{}"
	}
	return string(data)
}
