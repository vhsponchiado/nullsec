package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"nullsec/backend/internal/adapter/httpapi"
	"nullsec/backend/internal/adapter/sqliterepo"
	"nullsec/backend/internal/adapter/ws"
	"nullsec/backend/internal/infra/ca"
	"nullsec/backend/internal/infra/mitm"
	"nullsec/backend/internal/infra/upstream"
	"nullsec/backend/internal/infra/wordlist"
	"nullsec/backend/internal/usecase/certificate"
	"nullsec/backend/internal/usecase/intruder"
	"nullsec/backend/internal/usecase/portscan"
	"nullsec/backend/internal/usecase/proxy"
	"nullsec/backend/internal/usecase/repeater"
	"nullsec/backend/internal/usecase/settings"
	"nullsec/backend/internal/usecase/spider"
	"nullsec/backend/internal/usecase/sqliscan"
	"nullsec/backend/internal/usecase/subdomain"
	"nullsec/backend/internal/usecase/xssscan"
)

const (
	defaultProxyPort = "8080"
	defaultAPIPort   = "8081"

	// shutdownTimeout bounds how long we wait for in-flight requests to
	// drain on SIGINT/SIGTERM before forcing both listeners closed.
	shutdownTimeout = 5 * time.Second

	// These bound how long a connection may take across its read, write and
	// idle-keepalive phases. Without them, a client that opens a connection
	// and trickles bytes in slowly (Slowloris-style) — or never sends a
	// closing byte — ties up a goroutine and file descriptor forever. This
	// matters more than usual here: the proxy listener accepts connections
	// from whatever the user points their browser or curl at.
	httpReadTimeout       = 30 * time.Second
	httpReadHeaderTimeout = 10 * time.Second
	httpWriteTimeout      = 60 * time.Second
	httpIdleTimeout       = 120 * time.Second
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func defaultNullsecDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".nullsec"
	}
	return filepath.Join(home, ".nullsec")
}

func defaultCACertPath() string {
	return filepath.Join(defaultNullsecDir(), "ca-cert.pem")
}

func defaultCAKeyPath() string {
	return filepath.Join(defaultNullsecDir(), "ca-key.pem")
}

// defaultDBPath keeps the SQLite file next to the CA material in ~/.nullsec
// rather than the current working directory. Proxy history can contain live
// cookies and auth headers captured from real browsing sessions, so it
// should never default to landing inside a git checkout.
func defaultDBPath() string {
	return filepath.Join(defaultNullsecDir(), "nullsec.db")
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       httpReadTimeout,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
}

// main is the composition root: the one place allowed to know about every
// layer (entities, use cases, adapters, infra) and wire concrete
// implementations into the interfaces (ports) each use case declares.
func main() {
	if len(os.Args) > 1 && os.Args[1] == "cert" {
		runCertCommand(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "chrome" {
		runChromeCommand(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		runVersionCommand()
		return
	}

	proxyPort := getenv("PROXY_PORT", defaultProxyPort)
	apiPort := getenv("API_PORT", defaultAPIPort)
	dbPath := getenv("DB_PATH", defaultDBPath())
	caCertPath := getenv("CA_CERT_PATH", defaultCACertPath())
	caKeyPath := getenv("CA_KEY_PATH", defaultCAKeyPath())

	history, err := sqliterepo.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer history.Close()

	certAuthority, err := ca.LoadOrCreate(caCertPath, caKeyPath)
	if err != nil {
		log.Fatalf("failed to load or create CA: %v", err)
	}
	log.Printf("CA certificate at %s (download via GET /api/ca/cert)", caCertPath)

	hub := ws.NewHub()
	interceptor := proxy.NewInterceptor()
	responseInterceptor := proxy.NewResponseInterceptor()
	upstreamMgr := upstream.NewManager()

	forwarder := proxy.NewForwarder(history, hub, interceptor, responseInterceptor, mitm.NewTransport())
	mitmEngine := mitm.NewEngine(certAuthority, forwarder)
	proxySrv := newServer(":"+proxyPort, mitmEngine)

	intruderMgr := intruder.NewManager(hub, upstreamMgr, wordlist.Adapter{})
	portscanMgr := portscan.NewManager(hub, upstreamMgr)
	subdomainMgr := subdomain.NewManager(hub, upstreamMgr)
	spiderMgr := spider.NewManager(hub, history, upstreamMgr)
	certificateUC := certificate.New(certAuthority)
	settingsUC := settings.New(upstreamMgr)
	repeaterUC := repeater.New(upstreamMgr, forwarder)
	xssscanMgr := xssscan.NewManager(hub, history)
	sqliscanMgr := sqliscan.NewManager(hub, history)

	apiMux := http.NewServeMux()
	apiMux.Handle("/api/", httpapi.NewHandler(history, interceptor, responseInterceptor, certificateUC, intruderMgr, portscanMgr, subdomainMgr, spiderMgr, settingsUC, repeaterUC, xssscanMgr, sqliscanMgr))
	apiMux.HandleFunc("/ws", hub.ServeHTTP)

	// STATIC_DIR is unset for local development, where the frontend runs on
	// its own via `npm run dev` — set only in the Docker image, where the
	// frontend's build output is baked in and served same-origin so the
	// frontend needs no build-time API/WS base URL at all (see Dockerfile).
	// "/api/" and "/ws" above stay more specific and win over this "/".
	if staticDir := os.Getenv("STATIC_DIR"); staticDir != "" {
		apiMux.Handle("/", http.FileServer(http.Dir(staticDir)))
		log.Printf("serving frontend static files from %s", staticDir)
	}

	apiSrv := newServer(":"+apiPort, withCORS(apiMux))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		log.Printf("proxy listening on :%s", proxyPort)
		if err := proxySrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("proxy server: %w", err)
		}
	}()
	go func() {
		log.Printf("api/ws listening on :%s", apiPort)
		if err := apiSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("shutting down...")
	case err := <-errCh:
		log.Println(err)
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	_ = proxySrv.Shutdown(shutdownCtx)
	_ = apiSrv.Shutdown(shutdownCtx)
}
