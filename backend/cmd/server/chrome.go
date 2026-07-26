package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"nullsec/backend/internal/adapter/chromedp"
	chromeuc "nullsec/backend/internal/usecase/chrome"
)

// runChromeCommand implements `nullsec-server chrome [url]`: opens a
// throwaway Chrome instance pre-configured to use the proxy and ignore
// certificate errors, replacing the old chrome-proxy.sh script with a
// proper Go implementation (see internal/usecase/chrome and
// internal/adapter/chromedp).
func runChromeCommand(args []string) {
	fs := flag.NewFlagSet("chrome", flag.ExitOnError)
	proxyHost := fs.String("proxy-host", "127.0.0.1", "Nullsec proxy host")
	proxyPort := fs.String("proxy-port", getenv("PROXY_PORT", "8080"), "Nullsec proxy port")
	loadExtensions := fs.Bool("load-extensions", true, "Sideload extensions from --real-profile")
	realProfile := fs.String("real-profile", chromedp.DefaultRealProfileDir(), "Real Chrome profile to sideload extensions from")
	_ = fs.Parse(args) // flag.ExitOnError already exits the process on a parse error

	startURL := "about:blank"
	if fs.NArg() > 0 {
		startURL = fs.Arg(0)
	}

	cfg := chromeuc.Config{
		ProxyServer:    *proxyHost + ":" + *proxyPort,
		StartURL:       startURL,
		LoadExtensions: *loadExtensions,
		RealProfileDir: *realProfile,
	}

	fmt.Printf("Starting Chrome with proxy %s (throwaway profile, certificate errors ignored)\n", cfg.ProxyServer)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := chromedp.Launch(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "chrome:", err)
		os.Exit(1)
	}
}
