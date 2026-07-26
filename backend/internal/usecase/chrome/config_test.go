package chrome_test

import (
	"testing"

	"nullsec/backend/internal/usecase/chrome"
)

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     chrome.Config
		wantErr bool
	}{
		{"missing proxy", chrome.Config{}, true},
		{"malformed proxy", chrome.Config{ProxyServer: "not-a-host-port"}, true},
		{"valid", chrome.Config{ProxyServer: "127.0.0.1:8080"}, false},
		{"extensions without profile dir", chrome.Config{ProxyServer: "127.0.0.1:8080", LoadExtensions: true}, true},
		{"extensions with profile dir", chrome.Config{ProxyServer: "127.0.0.1:8080", LoadExtensions: true, RealProfileDir: "/tmp"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}
