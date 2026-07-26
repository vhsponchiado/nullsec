package chromedp

import (
	"context"
	"os/exec"
	"testing"
	"time"

	chromeuc "nullsec/backend/internal/usecase/chrome"
)

func requireChromeBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("google-chrome"); err != nil {
		if _, err := exec.LookPath("chromium"); err != nil {
			t.Skip("no google-chrome/chromium binary on PATH")
		}
	}
}

// TestLaunchHeadless launches a real (headless) Chrome instance and confirms
// Launch returns once the context is cancelled, without leaking the
// temporary profile directory. It doesn't need a live proxy since it never
// navigates anywhere but about:blank.
func TestLaunchHeadless(t *testing.T) {
	requireChromeBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg := chromeuc.Config{
		ProxyServer: "127.0.0.1:59999",
		StartURL:    "about:blank",
		Headless:    true,
	}

	if err := Launch(ctx, cfg); err != nil {
		t.Fatalf("Launch returned an error: %v", err)
	}
}
