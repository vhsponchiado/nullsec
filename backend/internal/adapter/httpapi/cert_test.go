package httpapi_test

import (
	"bytes"
	"net/http"
	"testing"
)

func TestCert_Download(t *testing.T) {
	h := newHarness(t)

	rec := h.do(http.MethodGet, "/api/ca/cert", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-pem-file" {
		t.Fatalf("Content-Type = %q, want application/x-pem-file", ct)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("BEGIN CERTIFICATE")) {
		t.Fatalf("body doesn't look like a PEM certificate: %s", rec.Body.String())
	}
}

func TestCert_Regenerate(t *testing.T) {
	h := newHarness(t)

	before := h.do(http.MethodGet, "/api/ca/cert", nil).Body.Bytes()

	rec := h.do(http.MethodPost, "/api/ca/regenerate", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("regenerate status = %d", rec.Code)
	}

	after := h.do(http.MethodGet, "/api/ca/cert", nil).Body.Bytes()
	if bytes.Equal(before, after) {
		t.Fatal("regenerate should produce a different CA certificate")
	}
}
