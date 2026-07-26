package httpapi_test

import (
	"net/http"
	"testing"
)

func TestSettings_GetDefault(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/settings/upstream-proxy", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var cfg struct {
		Type string `json:"type"`
	}
	decode(t, rec, &cfg)
	if cfg.Type != "none" {
		t.Fatalf("default upstream type = %q, want none", cfg.Type)
	}
}

func TestSettings_SetAndGet(t *testing.T) {
	h := newHarness(t)

	rec := h.doJSON(http.MethodPost, "/api/settings/upstream-proxy", map[string]any{
		"type":    "socks5",
		"address": "127.0.0.1:9050",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("set status = %d body=%s", rec.Code, rec.Body.String())
	}

	getRec := h.do(http.MethodGet, "/api/settings/upstream-proxy", nil)
	var cfg struct {
		Type    string `json:"type"`
		Address string `json:"address"`
	}
	decode(t, getRec, &cfg)
	if cfg.Type != "socks5" || cfg.Address != "127.0.0.1:9050" {
		t.Fatalf("cfg = %+v, want the just-set socks5 config", cfg)
	}
}

func TestSettings_SetInvalidAddress(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/settings/upstream-proxy", map[string]any{
		"type":    "socks5",
		"address": "not-a-host-port",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a malformed address", rec.Code)
	}
}

func TestSettings_SetUnknownType(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/settings/upstream-proxy", map[string]any{"type": "carrier-pigeon"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unknown proxy type", rec.Code)
	}
}
