// Package upstream is the outbound-dialing driver behind
// entity.UpstreamProxyConfig: it actually opens the SOCKS5/HTTP-CONNECT
// tunnels (or dials directly, if unconfigured) that Repeater, Intruder, the
// Port Scanner, and the Subdomain Finder's crt.sh lookup send their traffic
// through.
//
// DNS lookups (used by the Subdomain Finder's brute-force step) are NOT
// routed through this proxy — that's a fundamental limitation of SOCKS5/HTTP
// proxying, not a nullsec-specific gap.
package upstream

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"nullsec/backend/internal/entity"
)

type Manager struct {
	mu  sync.RWMutex
	cfg entity.UpstreamProxyConfig
}

func NewManager() *Manager {
	return &Manager{cfg: entity.UpstreamProxyConfig{Type: entity.UpstreamNone}}
}

func (m *Manager) Get() entity.UpstreamProxyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) Set(cfg entity.UpstreamProxyConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	return nil
}

// DialContext opens a raw TCP connection to addr through the configured
// upstream proxy (or directly, if none is configured). Used by the Port
// Scanner, which needs to test arbitrary TCP ports rather than issue HTTP
// requests.
func (m *Manager) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	cfg := m.Get()
	switch cfg.Type {
	case entity.UpstreamSOCKS5:
		return m.dialSOCKS5(ctx, cfg, network, addr)
	case entity.UpstreamHTTP:
		return dialViaHTTPConnect(ctx, cfg, addr)
	default:
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
}

func (m *Manager) dialSOCKS5(ctx context.Context, cfg entity.UpstreamProxyConfig, network, addr string) (net.Conn, error) {
	var auth *proxy.Auth
	if cfg.Username != "" {
		auth = &proxy.Auth{User: cfg.Username, Password: cfg.Password}
	}
	d, err := proxy.SOCKS5(network, cfg.Address, auth, proxy.Direct)
	if err != nil {
		return nil, err
	}
	if cd, ok := d.(proxy.ContextDialer); ok {
		return cd.DialContext(ctx, network, addr)
	}
	return d.Dial(network, addr)
}

// dialViaHTTPConnect opens a raw TCP tunnel to addr by issuing an HTTP
// CONNECT request to the configured HTTP proxy — the same mechanism
// browsers use to tunnel HTTPS through a forward proxy, reused here to
// tunnel arbitrary TCP for port scanning. Many proxies restrict CONNECT to
// port 443; if this one does, it'll reject with a non-200 status.
func dialViaHTTPConnect(ctx context.Context, cfg entity.UpstreamProxyConfig, addr string) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", cfg.Address)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}
	if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT to %s failed: %s", addr, resp.Status)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

// HTTPClient returns an *http.Client whose requests go through the
// configured upstream proxy — used by Repeater, Intruder, and the
// Subdomain Finder's crt.sh lookup.
func (m *Manager) HTTPClient(timeout time.Duration) *http.Client {
	cfg := m.Get()
	transport := &http.Transport{}

	switch cfg.Type {
	case entity.UpstreamHTTP:
		proxyURL := &url.URL{Scheme: "http", Host: cfg.Address}
		if cfg.Username != "" {
			proxyURL.User = url.UserPassword(cfg.Username, cfg.Password)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	case entity.UpstreamSOCKS5:
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return m.dialSOCKS5(ctx, cfg, network, addr)
		}
	}

	return &http.Client{Timeout: timeout, Transport: transport}
}
