// Package chrome holds the config for a single throwaway Chrome launch —
// used by the `chrome` CLI subcommand (cmd/server/chrome.go), which calls
// adapter/chromedp.Launch directly with it.
package chrome

import (
	"errors"
	"fmt"
	"net"
)

// Config configures a single throwaway Chrome launch.
type Config struct {
	// ProxyServer is the host:port of the Nullsec proxy, e.g. "127.0.0.1:8080".
	ProxyServer string
	// StartURL is the page Chrome opens on launch. Defaults to "about:blank".
	StartURL string
	// LoadExtensions, when true, sideloads extensions found in
	// RealProfileDir into the throwaway profile. Only the extension files
	// come along — cookies, history and login state never do.
	LoadExtensions bool
	// RealProfileDir is the real Chrome profile to read extensions from,
	// e.g. "$HOME/.config/google-chrome/Default". Required only when
	// LoadExtensions is true.
	RealProfileDir string
	// Headless runs Chrome without a visible window. Only useful for
	// automated smoke tests — actual interception needs a window to browse
	// in, so leave this false for real use.
	Headless bool
}

func (c Config) Validate() error {
	if c.ProxyServer == "" {
		return errors.New("chrome: ProxyServer is required")
	}
	if _, _, err := net.SplitHostPort(c.ProxyServer); err != nil {
		return fmt.Errorf("chrome: invalid ProxyServer %q: expected host:port", c.ProxyServer)
	}
	if c.LoadExtensions && c.RealProfileDir == "" {
		return errors.New("chrome: RealProfileDir is required when LoadExtensions is set")
	}
	return nil
}
