// Package portscan implements an nmap-style TCP connect scanner: for each
// port, it attempts a real TCP handshake (no raw sockets / SYN scan — that
// needs root and isn't portable in Go) and classifies the result as open,
// closed or filtered, grabbing a banner from anything that answers.
//
// This makes real connections against the target — only use it against
// hosts you're authorized to test.
package portscan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nullsec/backend/internal/entity"
)

// Dialer opens outbound connections through whatever upstream proxy is
// currently configured (or directly, if none is). The only implementation
// is infra/upstream.Manager, but this use case doesn't know that.
type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

type PortInfo struct {
	Port    int    `json:"port"`
	Service string `json:"service"`
}

// Top100 and Top1000 are the most commonly open TCP ports, most frequent
// first (from nmap's nmap-services frequency data).
var (
	Top100  = topPorts[:100]
	Top1000 = topPorts
)

func serviceFor(port int) string {
	if name, ok := serviceNames[port]; ok {
		return name
	}
	return "unknown"
}

const (
	MinPort = 1
	MaxPort = 65535
)

type State string

const (
	StateOpen     State = "open"
	StateClosed   State = "closed"
	StateFiltered State = "filtered"
)

type Result struct {
	Port       int    `json:"port"`
	State      State  `json:"state"`
	Service    string `json:"service"`
	Banner     string `json:"banner,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

const (
	defaultConcurrency = 200
	maxConcurrency     = 500
	defaultTimeout     = 800 * time.Millisecond
	minTimeoutMs       = 100
	maxTimeoutMs       = 5000
	bannerReadLimit    = 256
	bannerReadTimeout  = 300 * time.Millisecond
)

type StartRequest struct {
	Host        string `json:"host"`
	Ports       []int  `json:"ports"`
	TimeoutMs   int    `json:"timeout_ms"`
	Concurrency int    `json:"concurrency"`
}

type scan struct {
	mu      sync.Mutex
	status  entity.ScanStatus
	total   int
	results []Result
	cancel  context.CancelFunc
}

type Manager struct {
	mu     sync.Mutex
	scans  map[string]*scan
	hub    entity.Broadcaster
	dialer Dialer
}

func NewManager(hub entity.Broadcaster, dialer Dialer) *Manager {
	return &Manager{scans: make(map[string]*scan), hub: hub, dialer: dialer}
}

func (m *Manager) Start(req StartRequest) (string, int, error) {
	host := strings.TrimSpace(req.Host)
	if host == "" {
		return "", 0, errors.New("host is required")
	}
	if len(req.Ports) == 0 {
		return "", 0, errors.New("at least one port is required")
	}
	for _, p := range req.Ports {
		if p < MinPort || p > MaxPort {
			return "", 0, fmt.Errorf("invalid port %d", p)
		}
	}

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	if concurrency > maxConcurrency {
		concurrency = maxConcurrency
	}

	timeout := defaultTimeout
	if req.TimeoutMs > 0 {
		ms := req.TimeoutMs
		if ms < minTimeoutMs {
			ms = minTimeoutMs
		}
		if ms > maxTimeoutMs {
			ms = maxTimeoutMs
		}
		timeout = time.Duration(ms) * time.Millisecond
	}

	ports := append([]int(nil), req.Ports...)
	sort.Ints(ports)

	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	s := &scan{status: entity.ScanRunning, total: len(ports), cancel: cancel}

	m.mu.Lock()
	m.scans[id] = s
	m.mu.Unlock()

	go m.run(ctx, id, s, host, ports, timeout, concurrency)

	return id, len(ports), nil
}

func (m *Manager) run(ctx context.Context, id string, s *scan, host string, ports []int, timeout time.Duration, concurrency int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

runLoop:
	for _, port := range ports {
		select {
		case <-ctx.Done():
			break runLoop
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(port int) {
			defer wg.Done()
			defer func() { <-sem }()

			result := probe(ctx, m.dialer, host, port, timeout)

			s.mu.Lock()
			s.results = append(s.results, result)
			s.mu.Unlock()

			m.hub.Broadcast(entity.EventPortScanResult, map[string]any{"scan_id": id, "result": result})
		}(port)
	}
	wg.Wait()

	s.mu.Lock()
	if s.status == entity.ScanRunning {
		s.status = entity.ScanDone
	}
	status := s.status
	s.mu.Unlock()

	eventType := entity.EventPortScanDone
	if status == entity.ScanStopped {
		eventType = entity.EventPortScanStopped
	}
	m.hub.Broadcast(eventType, map[string]any{"scan_id": id})
}

func probe(ctx context.Context, dialer Dialer, host string, port int, timeout time.Duration) Result {
	started := time.Now()
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	durationMs := time.Since(started).Milliseconds()

	if err != nil {
		state := StateFiltered
		var netErr net.Error
		if errors.As(err, &netErr) && !netErr.Timeout() && strings.Contains(err.Error(), "refused") {
			state = StateClosed
		}
		return Result{Port: port, State: state, Service: serviceFor(port), DurationMs: durationMs}
	}
	defer conn.Close()

	banner := grabBanner(conn)
	return Result{Port: port, State: StateOpen, Service: serviceFor(port), Banner: banner, DurationMs: durationMs}
}

func grabBanner(conn net.Conn) string {
	_ = conn.SetReadDeadline(time.Now().Add(bannerReadTimeout))
	buf := make([]byte, bannerReadLimit)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return ""
	}
	return sanitizeBanner(string(buf[:n]))
}

// sanitizeBanner strips control characters (besides basic whitespace) so a
// banner from an arbitrary service can't inject terminal escapes or break
// the results table's layout.
func sanitizeBanner(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 0x20 && r != 0x7f) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func (m *Manager) Stop(id string) bool {
	m.mu.Lock()
	s, ok := m.scans[id]
	m.mu.Unlock()
	if !ok {
		return false
	}
	s.mu.Lock()
	if s.status == entity.ScanRunning {
		s.status = entity.ScanStopped
	}
	s.mu.Unlock()
	s.cancel()
	return true
}

func (m *Manager) Snapshot(id string) (status entity.ScanStatus, total int, results []Result, ok bool) {
	m.mu.Lock()
	s, ok := m.scans[id]
	m.mu.Unlock()
	if !ok {
		return "", 0, nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A nil slice marshals to JSON `null`, not `[]` — always return a
	// non-nil slice so the frontend never has to guard against that.
	results = make([]Result, len(s.results))
	copy(results, s.results)
	return s.status, s.total, results, true
}
