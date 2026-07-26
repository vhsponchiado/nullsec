package repeater

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeHTTPClientProvider struct {
	client *http.Client
}

func (f *fakeHTTPClientProvider) HTTPClient(time.Duration) *http.Client { return f.client }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func clientReturning(status int, body string, err error) *fakeHTTPClientProvider {
	return &fakeHTTPClientProvider{client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": {"text/plain"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}}
}

type fakeForwarder struct {
	status   int
	headers  http.Header
	body     []byte
	dropped  bool
	timedOut bool
	err      error
}

func (f *fakeForwarder) Send(r *http.Request, forceResponseIntercept bool) (int, http.Header, []byte, bool, bool, error) {
	return f.status, f.headers, f.body, f.dropped, f.timedOut, f.err
}

func TestSend_Direct_Success(t *testing.T) {
	uc := New(clientReturning(200, "hello", nil), &fakeForwarder{})

	resp := uc.Send(Request{Method: "GET", URL: "http://example.com/"})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.Status != 200 || resp.Body != "hello" || resp.Size != len("hello") {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestSend_Direct_TransportError(t *testing.T) {
	uc := New(clientReturning(0, "", errors.New("connection refused")), &fakeForwarder{})

	resp := uc.Send(Request{Method: "GET", URL: "http://example.com/"})
	if resp.Error == "" {
		t.Fatal("expected a non-empty Error for a failed round trip")
	}
}

func TestSend_Direct_InvalidMethod(t *testing.T) {
	uc := New(clientReturning(200, "", nil), &fakeForwarder{})

	// A method containing a space is rejected by http.NewRequest itself
	// before anything hits the network.
	resp := uc.Send(Request{Method: "BAD METHOD", URL: "http://example.com/"})
	if resp.Error == "" {
		t.Fatal("expected a non-empty Error for an invalid method")
	}
}

func TestSend_ViaProxy_Success(t *testing.T) {
	forwarder := &fakeForwarder{status: 201, headers: http.Header{"X-Test": {"1"}}, body: []byte("held")}
	uc := New(clientReturning(0, "", nil), forwarder)

	resp := uc.Send(Request{Method: "GET", URL: "http://example.com/", ViaProxy: true})
	if resp.Status != 201 || resp.Body != "held" || resp.Size != len("held") {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestSend_ViaProxy_Dropped(t *testing.T) {
	forwarder := &fakeForwarder{dropped: true}
	uc := New(clientReturning(0, "", nil), forwarder)

	resp := uc.Send(Request{Method: "GET", URL: "http://example.com/", ViaProxy: true})
	if !resp.Dropped {
		t.Fatalf("resp.Dropped = false, want true, got %+v", resp)
	}
}

func TestSend_ViaProxy_TimedOut(t *testing.T) {
	forwarder := &fakeForwarder{timedOut: true}
	uc := New(clientReturning(0, "", nil), forwarder)

	resp := uc.Send(Request{Method: "GET", URL: "http://example.com/", ViaProxy: true})
	if !resp.TimedOut {
		t.Fatalf("resp.TimedOut = false, want true, got %+v", resp)
	}
}

func TestSend_ViaProxy_Error(t *testing.T) {
	forwarder := &fakeForwarder{err: errors.New("boom")}
	uc := New(clientReturning(0, "", nil), forwarder)

	resp := uc.Send(Request{Method: "GET", URL: "http://example.com/", ViaProxy: true})
	if resp.Error == "" {
		t.Fatal("expected a non-empty Error")
	}
}
