package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"nullsec/backend/internal/entity"
)

func TestForward_NoIntercept_Success(t *testing.T) {
	history := &fakeHistory{}
	hub := &fakeBroadcaster{}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("outbound method = %q, want GET", r.Method)
		}
		if r.URL.String() != "http://example.com/hello" {
			t.Fatalf("outbound URL = %q", r.URL.String())
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("hello world")),
		}, nil
	})

	f := NewForwarder(history, hub, NewInterceptor(), NewResponseInterceptor(), transport)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/hello", nil)
	result := f.Forward(req, false)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Dropped || result.TimedOut {
		t.Fatalf("got Dropped=%v TimedOut=%v, want both false", result.Dropped, result.TimedOut)
	}
	if result.Status != 200 {
		t.Fatalf("Status = %d, want 200", result.Status)
	}
	if string(result.Body) != "hello world" {
		t.Fatalf("Body = %q, want %q", result.Body, "hello world")
	}

	items := history.all()
	if len(items) != 1 {
		t.Fatalf("history has %d items, want 1", len(items))
	}
	rec := items[0]
	if rec.Method != "GET" || rec.URL != "http://example.com/hello" || rec.Host != "example.com" {
		t.Fatalf("recorded request = %+v", rec)
	}
	if rec.Status != 200 || rec.Outcome != "completed" || rec.Size != len("hello world") {
		t.Fatalf("recorded outcome = %+v", rec)
	}

	events := hub.events()
	if len(events) != 1 || events[0].eventType != entity.EventRequestCompleted {
		t.Fatalf("broadcast events = %+v, want a single EventRequestCompleted", events)
	}
	// This also guards the fix that stopped the full Request (with its
	// potentially large headers/body/resp_body) from being broadcast over
	// the WS on every proxied request — only the lightweight summary should
	// go out.
	summary, ok := events[0].payload.(entity.RequestSummary)
	if !ok {
		t.Fatalf("EventRequestCompleted payload type = %T, want entity.RequestSummary", events[0].payload)
	}
	if summary.ID != rec.ID || summary.Status != 200 {
		t.Fatalf("broadcast summary = %+v, want it to match the saved record", summary)
	}
}

func TestForward_TransportError_StillRecordsHistory(t *testing.T) {
	history := &fakeHistory{}
	hub := &fakeBroadcaster{}
	wantErr := errors.New("connection refused")
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, wantErr
	})
	f := NewForwarder(history, hub, NewInterceptor(), NewResponseInterceptor(), transport)

	req := httptest.NewRequest(http.MethodGet, "http://unreachable.example/", nil)
	result := f.Forward(req, false)

	if result.Err == nil {
		t.Fatal("expected the transport error to be surfaced on ForwardResult.Err")
	}

	items := history.all()
	if len(items) != 1 {
		t.Fatalf("history has %d items, want 1 (a failed round trip is still recorded)", len(items))
	}
	if items[0].Status != 0 {
		t.Fatalf("recorded Status = %d, want 0 for a round trip that never got a response", items[0].Status)
	}
}

func TestForward_Intercept_ForwardWithEdits(t *testing.T) {
	history := &fakeHistory{}
	interceptor := NewInterceptor()
	interceptor.SetEnabled(true)

	var capturedReq *http.Request
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedReq = r
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})

	hub := &fakeBroadcaster{}
	hub.onBroadcast = func(eventType string, payload any) {
		if eventType != entity.EventRequestIntercepted {
			return
		}
		pending := payload.(PendingRequest)
		interceptor.Resolve(pending.ID, Decision{
			Action:  "forward",
			Method:  http.MethodPost,
			URL:     "http://edited.example/new-path",
			Headers: http.Header{"X-Edited": {"yes"}},
			Body:    "edited body",
		})
	}

	f := NewForwarder(history, hub, interceptor, NewResponseInterceptor(), transport)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/original", strings.NewReader("original body"))
	result := f.Forward(req, false)

	if result.Err != nil || result.Dropped {
		t.Fatalf("unexpected result: %+v", result)
	}
	if capturedReq == nil {
		t.Fatal("transport was never called")
	}
	if capturedReq.Method != http.MethodPost {
		t.Fatalf("outbound method = %q, want POST (the edited method)", capturedReq.Method)
	}
	if capturedReq.URL.String() != "http://edited.example/new-path" {
		t.Fatalf("outbound URL = %q, want the edited URL", capturedReq.URL.String())
	}
	if got := capturedReq.Header.Get("X-Edited"); got != "yes" {
		t.Fatalf("outbound headers = %v, missing the edited X-Edited header", capturedReq.Header)
	}
	body, _ := io.ReadAll(capturedReq.Body)
	if string(body) != "edited body" {
		t.Fatalf("outbound body = %q, want the edited body", body)
	}

	items := history.all()
	if len(items) != 1 || items[0].Body != "edited body" || items[0].Method != http.MethodPost {
		t.Fatalf("recorded history = %+v, want the edited values persisted", items)
	}

	var sawForwarded bool
	for _, e := range hub.events() {
		if e.eventType == entity.EventRequestForwarded {
			sawForwarded = true
		}
	}
	if !sawForwarded {
		t.Fatal("missing EventRequestForwarded broadcast")
	}
}

func TestForward_Intercept_Drop_NeverCallsTransport(t *testing.T) {
	history := &fakeHistory{}
	interceptor := NewInterceptor()
	interceptor.SetEnabled(true)

	var transportCalls int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&transportCalls, 1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
	})

	hub := &fakeBroadcaster{}
	hub.onBroadcast = func(eventType string, payload any) {
		if eventType != entity.EventRequestIntercepted {
			return
		}
		pending := payload.(PendingRequest)
		interceptor.Resolve(pending.ID, Decision{Action: "drop"})
	}

	f := NewForwarder(history, hub, interceptor, NewResponseInterceptor(), transport)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/secret", nil)
	result := f.Forward(req, false)

	if !result.Dropped {
		t.Fatal("result.Dropped = false, want true")
	}
	if atomic.LoadInt32(&transportCalls) != 0 {
		t.Fatal("a dropped request must never reach the transport/network")
	}

	items := history.all()
	if len(items) != 1 || items[0].Outcome != "dropped" {
		t.Fatalf("recorded history = %+v, want a single dropped entry", items)
	}

	events := hub.events()
	if len(events) != 2 || events[0].eventType != entity.EventRequestIntercepted || events[1].eventType != entity.EventRequestDropped {
		t.Fatalf("event sequence = %+v, want [Intercepted, Dropped]", events)
	}
}

func TestForward_Intercept_Timeout(t *testing.T) {
	original := interceptTimeout
	interceptTimeout = 20 * time.Millisecond
	defer func() { interceptTimeout = original }()

	history := &fakeHistory{}
	interceptor := NewInterceptor()
	interceptor.SetEnabled(true)
	hub := &fakeBroadcaster{} // no hook — the pending request is deliberately left unresolved

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("the transport must never be called when intercept times out")
		return nil, nil
	})

	f := NewForwarder(history, hub, interceptor, NewResponseInterceptor(), transport)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/never-resolved", nil)
	result := f.Forward(req, false)

	if !result.TimedOut {
		t.Fatal("result.TimedOut = false, want true after interceptTimeout elapses")
	}

	items := history.all()
	if len(items) != 1 || items[0].Outcome != "dropped" {
		t.Fatalf("recorded history = %+v, want a single dropped entry", items)
	}
	if _, ok := interceptor.Get(items[0].ID); ok {
		t.Fatal("Forward's timeout path should Cleanup the pending entry")
	}

	var sawTimeoutDrop bool
	for _, e := range hub.events() {
		if e.eventType != entity.EventRequestDropped {
			continue
		}
		if m, ok := e.payload.(map[string]string); ok && m["reason"] == "timeout" {
			sawTimeoutDrop = true
		}
	}
	if !sawTimeoutDrop {
		t.Fatal("missing EventRequestDropped with reason=timeout")
	}
}

func TestForward_ResponseIntercept_ForwardWithEdits(t *testing.T) {
	history := &fakeHistory{}
	responseInterceptor := NewResponseInterceptor()
	responseInterceptor.SetEnabled(true)

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("original response")),
		}, nil
	})

	hub := &fakeBroadcaster{}
	hub.onBroadcast = func(eventType string, payload any) {
		if eventType != entity.EventResponseIntercepted {
			return
		}
		pending := payload.(PendingResponse)
		responseInterceptor.Resolve(pending.ID, ResponseDecision{
			Action:  "forward",
			Status:  201,
			Headers: http.Header{"Content-Type": {"text/plain"}},
			Body:    "edited response",
		})
	}

	f := NewForwarder(history, hub, NewInterceptor(), responseInterceptor, transport)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/page", nil)
	result := f.Forward(req, false)

	if result.Status != 201 {
		t.Fatalf("Status = %d, want 201 (the edited status)", result.Status)
	}
	if string(result.Body) != "edited response" {
		t.Fatalf("Body = %q, want the edited body", result.Body)
	}
	if got := result.Headers.Get("Content-Length"); got != "15" {
		t.Fatalf("Content-Length header = %q, want it recalculated to match the edited body (15 bytes)", got)
	}

	items := history.all()
	if len(items) != 1 || items[0].RespBody != "edited response" || items[0].Status != 201 {
		t.Fatalf("recorded history = %+v, want the edited response persisted", items)
	}
}

func TestForward_ResponseIntercept_Drop(t *testing.T) {
	history := &fakeHistory{}
	responseInterceptor := NewResponseInterceptor()
	responseInterceptor.SetEnabled(true)

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("body"))}, nil
	})

	hub := &fakeBroadcaster{}
	hub.onBroadcast = func(eventType string, payload any) {
		if eventType != entity.EventResponseIntercepted {
			return
		}
		pending := payload.(PendingResponse)
		responseInterceptor.Resolve(pending.ID, ResponseDecision{Action: "drop"})
	}

	f := NewForwarder(history, hub, NewInterceptor(), responseInterceptor, transport)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/page", nil)
	result := f.Forward(req, false)

	if !result.Dropped {
		t.Fatal("result.Dropped = false, want true")
	}
	items := history.all()
	if len(items) != 1 || items[0].Outcome != "dropped" {
		t.Fatalf("recorded history = %+v, want a single dropped entry", items)
	}
}

// TestForward_ForceResponseIntercept_OverridesDisabledGlobalToggle guards the
// Repeater "send to intercept-response" feature: forceResponseIntercept must
// hold the response even though the global response-intercept toggle is off.
func TestForward_ForceResponseIntercept_OverridesDisabledGlobalToggle(t *testing.T) {
	history := &fakeHistory{}
	responseInterceptor := NewResponseInterceptor() // left disabled

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("body"))}, nil
	})

	hub := &fakeBroadcaster{}
	hub.onBroadcast = func(eventType string, payload any) {
		if eventType != entity.EventResponseIntercepted {
			return
		}
		pending := payload.(PendingResponse)
		responseInterceptor.Resolve(pending.ID, ResponseDecision{Action: "forward", Status: 200, Body: "body"})
	}

	f := NewForwarder(history, hub, NewInterceptor(), responseInterceptor, transport)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/page", nil)
	result := f.Forward(req, true)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	var sawIntercepted bool
	for _, e := range hub.events() {
		if e.eventType == entity.EventResponseIntercepted {
			sawIntercepted = true
		}
	}
	if !sawIntercepted {
		t.Fatal("forceResponseIntercept=true should hold the response even though responseInterceptor.Enabled()=false")
	}
}

func TestSend_DelegatesToForward(t *testing.T) {
	history := &fakeHistory{}
	hub := &fakeBroadcaster{}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	f := NewForwarder(history, hub, NewInterceptor(), NewResponseInterceptor(), transport)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	status, _, _, dropped, timedOut, err := f.Send(req, false)

	if err != nil || dropped || timedOut || status != 204 {
		t.Fatalf("Send = status=%d dropped=%v timedOut=%v err=%v", status, dropped, timedOut, err)
	}
}

func TestCloneHeader(t *testing.T) {
	original := http.Header{"X-Foo": {"bar"}}
	clone := cloneHeader(original)

	clone.Set("X-Foo", "mutated")
	if original.Get("X-Foo") != "bar" {
		t.Fatal("cloneHeader must deep-copy — mutating the clone changed the original")
	}
}

func TestHeadersToJSON(t *testing.T) {
	if got := headersToJSON(nil); got != "{}" {
		t.Fatalf("headersToJSON(nil) = %q, want %q", got, "{}")
	}
	if got := headersToJSON(http.Header{"X-Foo": {"bar"}}); got == "" || got == "{}" {
		t.Fatalf("headersToJSON with a real header should not be empty/{}, got %q", got)
	}
}
