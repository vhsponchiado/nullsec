package mitm

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type countingRoundTripper struct {
	calls   int
	results []func() (*http.Response, error)
}

func (c *countingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	i := c.calls
	c.calls++
	if i < len(c.results) {
		return c.results[i]()
	}
	return c.results[len(c.results)-1]()
}

func ok() (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}

func fail() (*http.Response, error) {
	return nil, errors.New("EOF")
}

func TestRetryOnceTransport_RetriesGETOnFailure(t *testing.T) {
	inner := &countingRoundTripper{results: []func() (*http.Response, error){fail, ok}}
	transport := &retryOnceTransport{rt: inner}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 from the retried attempt", resp.StatusCode)
	}
	if inner.calls != 2 {
		t.Fatalf("inner.calls = %d, want 2 (original + one retry)", inner.calls)
	}
}

func TestRetryOnceTransport_OnlyRetriesOnce(t *testing.T) {
	inner := &countingRoundTripper{results: []func() (*http.Response, error){fail, fail, ok}}
	transport := &retryOnceTransport{rt: inner}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Fatal("expected the second failure to be surfaced, not silently retried again")
	}
	if inner.calls != 2 {
		t.Fatalf("inner.calls = %d, want exactly 2 (no second retry)", inner.calls)
	}
}

func TestRetryOnceTransport_DoesNotRetryPOST(t *testing.T) {
	inner := &countingRoundTripper{results: []func() (*http.Response, error){fail, ok}}
	transport := &retryOnceTransport{rt: inner}

	req := httptest.NewRequest(http.MethodPost, "http://example.com/", nil)
	req.ContentLength = 0
	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Fatal("POST should not be retried even with no body, since it isn't idempotent")
	}
	if inner.calls != 1 {
		t.Fatalf("inner.calls = %d, want 1 (no retry)", inner.calls)
	}
}

func TestRetryOnceTransport_DoesNotRetryGETWithBody(t *testing.T) {
	inner := &countingRoundTripper{results: []func() (*http.Response, error){fail, ok}}
	transport := &retryOnceTransport{rt: inner}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.ContentLength = 10 // simulate a GET carrying a body
	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Fatal("a GET with a non-empty body should not be retried (the body reader may already be consumed)")
	}
	if inner.calls != 1 {
		t.Fatalf("inner.calls = %d, want 1 (no retry)", inner.calls)
	}
}

func TestRetryOnceTransport_NoRetryNeededOnSuccess(t *testing.T) {
	inner := &countingRoundTripper{results: []func() (*http.Response, error){ok}}
	transport := &retryOnceTransport{rt: inner}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inner.calls != 1 {
		t.Fatalf("inner.calls = %d, want 1", inner.calls)
	}
}
