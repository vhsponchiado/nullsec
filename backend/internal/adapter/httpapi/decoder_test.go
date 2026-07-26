package httpapi_test

import (
	"net/http"
	"testing"
)

func TestDecoder_Operations(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/decode-operations", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var ops []map[string]any
	decode(t, rec, &ops)
	if len(ops) == 0 {
		t.Fatal("expected a non-empty operations list")
	}
}

func TestDecoder_EncodeThenDecodeRoundTrip(t *testing.T) {
	h := newHarness(t)

	encRec := h.doJSON(http.MethodPost, "/api/tools/decode", map[string]any{
		"op": "base64", "mode": "encode", "input": "hello world",
	})
	if encRec.Code != http.StatusOK {
		t.Fatalf("encode status = %d body=%s", encRec.Code, encRec.Body.String())
	}
	var encoded struct {
		Output string `json:"output"`
	}
	decode(t, encRec, &encoded)
	if encoded.Output == "" || encoded.Output == "hello world" {
		t.Fatalf("encoded output = %q, doesn't look base64-encoded", encoded.Output)
	}

	decRec := h.doJSON(http.MethodPost, "/api/tools/decode", map[string]any{
		"op": "base64", "mode": "decode", "input": encoded.Output,
	})
	var decoded struct {
		Output string `json:"output"`
	}
	decode(t, decRec, &decoded)
	if decoded.Output != "hello world" {
		t.Fatalf("round trip = %q, want %q", decoded.Output, "hello world")
	}
}

func TestDecoder_InvalidMode(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/decode", map[string]any{
		"op": "base64", "mode": "sideways", "input": "x",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an invalid mode", rec.Code)
	}
}

func TestDecoder_UnknownOpReturnsErrorField(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/decode", map[string]any{
		"op": "not-a-real-op", "mode": "encode", "input": "x",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (errors from a bad op are reported in the body, not the status)", rec.Code)
	}
	var body struct {
		Error string `json:"error"`
	}
	decode(t, rec, &body)
	if body.Error == "" {
		t.Fatal("expected a non-empty error field for an unknown operation")
	}
}

func TestDecoder_SmartDecode(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/decode/smart", map[string]any{"input": "aGVsbG8="})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Steps []map[string]any `json:"steps"`
	}
	decode(t, rec, &body)
	if len(body.Steps) == 0 {
		t.Fatal("expected smart decode to detect at least one layer for a base64 string")
	}
}
