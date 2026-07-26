// Package chromedp drives a throwaway Google Chrome instance via chromedp
// (which talks to Chrome over the DevTools protocol), called directly by
// the `chrome` CLI subcommand (cmd/server/chrome.go). Using chromedp instead
// of shelling out to the `google-chrome` binary directly gives the launch
// proper lifecycle management: the profile directory is unique per launch
// and removed once Chrome exits, and cancelling the context — whether
// because the caller asked to stop, or because the user closed the window
// — is what tears everything down. There is no orphaned process or
// leftover profile directory to clean up by hand.
package chromedp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"

	chromeuc "nullsec/backend/internal/usecase/chrome"
)

// DefaultRealProfileDir returns the default Chrome profile directory to
// sideload extensions from when the caller doesn't specify one. It only
// covers Linux's default install layout — pass RealProfileDir explicitly on
// macOS/Windows, or if Chrome/Chromium isn't installed at its usual path.
func DefaultRealProfileDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "google-chrome", "Default")
}

// Launch runs a throwaway Chrome instance and blocks until it exits — either
// because the user closed the window, or because ctx was cancelled. The
// profile directory is created fresh on entry and always removed on return,
// even if Launch returns an error. Called directly by the `chrome` CLI
// subcommand.
func Launch(ctx context.Context, cfg chromeuc.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	startURL := cfg.StartURL
	if startURL == "" {
		startURL = "about:blank"
	}

	profileDir, err := os.MkdirTemp("", "nullsec-chrome-*")
	if err != nil {
		return fmt.Errorf("chrome: create profile dir: %w", err)
	}
	defer os.RemoveAll(profileDir)

	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.IgnoreCertErrors,
		chromedp.Flag("test-type", true), // suppresses the "ignore-certificate-errors" warning banner
		chromedp.UserDataDir(profileDir),
		chromedp.ProxyServer(cfg.ProxyServer),
		chromedp.Flag("proxy-bypass-list", "<-loopback>"),
	}
	if cfg.Headless {
		opts = append(opts, chromedp.Headless)
	}

	if cfg.LoadExtensions {
		paths, err := discoverExtensions(cfg.RealProfileDir)
		if err != nil {
			return fmt.Errorf("chrome: discover extensions: %w", err)
		}
		if len(paths) > 0 {
			opts = append(opts, chromedp.Flag("load-extension", joinPaths(paths)))
		}
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	if err := chromedp.Run(taskCtx, chromedp.Navigate(startURL)); err != nil {
		return fmt.Errorf("chrome: launch: %w", err)
	}

	// chromedp cancels taskCtx itself once it detects Chrome has exited (the
	// user closed the window), so this also unblocks on a graceful quit —
	// not just on the caller cancelling ctx.
	<-taskCtx.Done()

	return nil
}

// discoverExtensions finds the currently installed version of every
// extension in a Chrome profile's Extensions directory, returning one path
// per extension suitable for a comma-joined --load-extension flag. Each
// extension ID has one subdirectory per cached version; the most recently
// modified one (i.e. the one Chrome last unpacked) is taken as current.
func discoverExtensions(profileDir string) ([]string, error) {
	extRoot := filepath.Join(profileDir, "Extensions")

	entries, err := os.ReadDir(extRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		latest, err := latestVersionDir(filepath.Join(extRoot, entry.Name()))
		if err != nil {
			return nil, err
		}
		if latest != "" {
			paths = append(paths, latest)
		}
	}
	return paths, nil
}

// latestVersionDir returns the most recently modified version subdirectory
// of an extension's cache directory that has a manifest.json, or "" if none
// qualify.
func latestVersionDir(extDir string) (string, error) {
	entries, err := os.ReadDir(extDir)
	if err != nil {
		return "", err
	}

	var (
		best     string
		bestTime time.Time
	)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		versionDir := filepath.Join(extDir, entry.Name())
		if _, err := os.Stat(filepath.Join(versionDir, "manifest.json")); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestTime) {
			best = versionDir
			bestTime = info.ModTime()
		}
	}
	return best, nil
}

func joinPaths(paths []string) string {
	out := paths[0]
	for _, p := range paths[1:] {
		out += "," + p
	}
	return out
}
