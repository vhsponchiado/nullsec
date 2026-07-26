package mitm

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nullsec/backend/internal/entity"
	proxyuc "nullsec/backend/internal/usecase/proxy"
)

func TestEngine_ServeHTTP_RequiresAbsoluteURI(t *testing.T) {
	forwarder := proxyuc.NewForwarder(&fakeHistory{}, fakeBroadcaster{}, proxyuc.NewInterceptor(), proxyuc.NewResponseInterceptor(), roundTripFunc(okRoundTrip))
	engine := NewEngine(nil, forwarder)

	req := httptest.NewRequest(http.MethodGet, "/relative/path", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a relative-form request line", rec.Code)
	}
}

func TestEngine_ServeHTTP_ForwardsSuccessfully(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusCreated,
			Header:     http.Header{"X-From-Target": {"1"}},
			Body:       io.NopCloser(strings.NewReader("hello from target")),
		}, nil
	})
	forwarder := proxyuc.NewForwarder(&fakeHistory{}, fakeBroadcaster{}, proxyuc.NewInterceptor(), proxyuc.NewResponseInterceptor(), transport)
	engine := NewEngine(nil, forwarder)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/hello", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if rec.Header().Get("X-From-Target") != "1" {
		t.Fatalf("headers = %v, want the target's header relayed verbatim", rec.Header())
	}
	if rec.Body.String() != "hello from target" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestEngine_ServeHTTP_TransportErrorMapsTo502(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})
	forwarder := proxyuc.NewForwarder(&fakeHistory{}, fakeBroadcaster{}, proxyuc.NewInterceptor(), proxyuc.NewResponseInterceptor(), transport)
	engine := NewEngine(nil, forwarder)

	req := httptest.NewRequest(http.MethodGet, "http://unreachable.example/", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 when the outbound round trip fails", rec.Code)
	}
}

func TestEngine_ServeHTTP_DroppedMapsTo502(t *testing.T) {
	interceptor := proxyuc.NewInterceptor()
	interceptor.SetEnabled(true)
	hub := fakeBroadcaster{onBroadcast: func(eventType string, payload any) {
		if eventType != entity.EventRequestIntercepted {
			return
		}
		pending := payload.(proxyuc.PendingRequest)
		interceptor.Resolve(pending.ID, proxyuc.Decision{Action: "drop"})
	}}
	forwarder := proxyuc.NewForwarder(&fakeHistory{}, hub, interceptor, proxyuc.NewResponseInterceptor(), roundTripFunc(okRoundTrip))
	engine := NewEngine(nil, forwarder)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/secret", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 for a dropped request", rec.Code)
	}
}

func okRoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
}
