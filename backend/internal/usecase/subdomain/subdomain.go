// Package subdomain implements a subdomain finder combining two techniques:
//
//   - Passive: queries crt.sh (Certificate Transparency logs) for certificates
//     issued for *.domain — reveals real subdomains without ever touching the
//     target.
//   - Active: DNS brute-force, trying common subdomain prefixes (from a
//     SecLists wordlist, ranked by real-world frequency) and resolving each
//     candidate.
//
// This performs real DNS lookups (and one HTTPS request to crt.sh) — only
// use it against domains you're authorized to test.
package subdomain

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nullsec/backend/internal/entity"
)

// HTTPClientProvider hands back an *http.Client routed through whatever
// upstream proxy is currently configured — used for the crt.sh lookup. The
// only implementation is infra/upstream.Manager, but this use case doesn't
// know that.
type HTTPClientProvider interface {
	HTTPClient(timeout time.Duration) *http.Client
}

//go:embed subdomains.txt
var rawWordlist string

// Wordlist is the built-in list of common subdomain prefixes, most frequent
// first (SecLists' subdomains-top1million-5000.txt).
var Wordlist = strings.Split(strings.TrimSpace(rawWordlist), "\n")

const (
	MaxCandidates      = 20000
	defaultConcurrency = 50
	maxConcurrency     = 200
	defaultTimeout     = 3 * time.Second
	minTimeoutMs       = 200
	maxTimeoutMs       = 10000
	crtshTimeout       = 15 * time.Second
)

// NormalizeDomain strips a scheme, path, port and trailing slash from user
// input so pasting a full URL works the same as typing a bare domain.
func NormalizeDomain(input string) string {
	d := strings.ToLower(strings.TrimSpace(input))
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	if i := strings.IndexAny(d, "/:?#"); i >= 0 {
		d = d[:i]
	}
	return d
}

type crtshEntry struct {
	NameValue string `json:"name_value"`
}

// crtshRetries is how many times queryCRTSH tries the lookup before giving
// up. crt.sh is a free, community-run service with no SLA — it's common for
// a query to time out, get rate-limited, or (worse) return a 200 with a
// truncated/empty result under load, all without any indication it happened
// short of comparing against a previous run's results.
const crtshRetries = 2

// crtshRetryDelay is how long queryCRTSH waits between attempts. A var, not
// a const, so tests can shrink it rather than actually sleeping.
var crtshRetryDelay = 2 * time.Second

// queryCRTSH fetches every certificate name (including SANs) crt.sh has on
// record for *.domain, returning the deduped, lowercased hostnames. It
// retries on failure since a single crt.sh timeout/hiccup is common and
// otherwise silently drops every subdomain that was only ever discoverable
// through Certificate Transparency, not the brute-force wordlist.
func queryCRTSH(ctx context.Context, httpClients HTTPClientProvider, domain string) ([]string, error) {
	var lastErr error
	for attempt := 1; attempt <= crtshRetries; attempt++ {
		names, err := queryCRTSHOnce(ctx, httpClients, domain)
		if err == nil {
			return names, nil
		}
		lastErr = err

		if attempt < crtshRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(crtshRetryDelay):
			}
		}
	}
	return nil, lastErr
}

func queryCRTSHOnce(ctx context.Context, httpClients HTTPClientProvider, domain string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, crtshTimeout)
	defer cancel()

	reqURL := fmt.Sprintf("https://crt.sh/?q=%%.%s&output=json", domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClients.HTTPClient(crtshTimeout).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crt.sh returned status %d", resp.StatusCode)
	}

	var entries []crtshEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var out []string
	suffix := "." + domain
	for _, e := range entries {
		for _, line := range strings.Split(e.NameValue, "\n") {
			name := strings.ToLower(strings.TrimSpace(line))
			name = strings.TrimPrefix(name, "*.")
			if name != domain && !strings.HasSuffix(name, suffix) {
				continue
			}
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}

type Result struct {
	Subdomain  string   `json:"subdomain"`
	Resolved   bool     `json:"resolved"`
	IPs        []string `json:"ips,omitempty"`
	Sources    []string `json:"sources"`
	DurationMs int64    `json:"duration_ms"`
}

type StartRequest struct {
	Domain       string `json:"domain"`
	UseCRTSH     bool   `json:"use_crtsh"`
	WordlistSize int    `json:"wordlist_size"` // 0 disables brute-forcing
	Concurrency  int    `json:"concurrency"`
	TimeoutMs    int    `json:"timeout_ms"`
}

type job struct {
	mu      sync.Mutex
	status  entity.ScanStatus
	total   int
	results []Result
	cancel  context.CancelFunc
	warning string
}

type Manager struct {
	mu          sync.Mutex
	jobs        map[string]*job
	hub         entity.Broadcaster
	httpClients HTTPClientProvider
}

func NewManager(hub entity.Broadcaster, httpClients HTTPClientProvider) *Manager {
	return &Manager{jobs: make(map[string]*job), hub: hub, httpClients: httpClients}
}

func (m *Manager) Start(req StartRequest) (string, int, error) {
	domain := NormalizeDomain(req.Domain)
	if domain == "" {
		return "", 0, errors.New("domain is required")
	}
	if !req.UseCRTSH && req.WordlistSize <= 0 {
		return "", 0, errors.New("enable crt.sh, DNS brute-force, or both")
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

	wordlistSize := req.WordlistSize
	if wordlistSize > len(Wordlist) {
		wordlistSize = len(Wordlist)
	}

	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{status: entity.ScanRunning, cancel: cancel}

	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()

	go m.run(ctx, id, j, domain, req.UseCRTSH, wordlistSize, timeout, concurrency)

	// The candidate count (and therefore total) isn't known until crt.sh
	// responds, which happens inside run(); the caller gets the eventual
	// total via the first snapshot/WS updates instead of blocking here.
	return id, wordlistSize + 1, nil
}

func (m *Manager) run(ctx context.Context, id string, j *job, domain string, useCRTSH bool, wordlistSize int, timeout time.Duration, concurrency int) {
	candidates := make(map[string]map[string]bool)
	addCandidate := func(name, source string) {
		if candidates[name] == nil {
			candidates[name] = make(map[string]bool)
		}
		candidates[name][source] = true
	}

	addCandidate(domain, "root")

	if useCRTSH {
		names, err := queryCRTSH(ctx, m.httpClients, domain)
		switch {
		case err != nil:
			j.mu.Lock()
			j.warning = "crt.sh lookup failed: " + err.Error()
			j.mu.Unlock()
		case len(names) == 0:
			// crt.sh answered with no error but zero matching certificates.
			// That's sometimes a genuine result (a domain with no other
			// certs in Certificate Transparency logs), but it's also what a
			// silently-truncated or rate-limited response looks like — crt.sh
			// gives no way to tell those apart. Flag it so a run that looks
			// suspiciously thinner than a previous one isn't mistaken for
			// "these subdomains stopped existing."
			j.mu.Lock()
			j.warning = "crt.sh returned no certificates for this domain — this can be a genuine empty result, or crt.sh silently truncating/rate-limiting; re-run if you expected more here"
			j.mu.Unlock()
		default:
			for _, name := range names {
				addCandidate(name, "crtsh")
			}
		}
	}

	for i := 0; i < wordlistSize; i++ {
		addCandidate(Wordlist[i]+"."+domain, "bruteforce")
	}

	if len(candidates) > MaxCandidates {
		j.mu.Lock()
		j.status = entity.ScanDone
		j.warning = fmt.Sprintf("too many candidates (%d) — capped at %d; reduce the wordlist size", len(candidates), MaxCandidates)
		j.mu.Unlock()
		m.hub.Broadcast(entity.EventSubdomainDone, map[string]any{"scan_id": id})
		return
	}

	j.mu.Lock()
	j.total = len(candidates)
	j.mu.Unlock()
	m.hub.Broadcast(entity.EventSubdomainTotal, map[string]any{"scan_id": id, "total": len(candidates)})

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

runLoop:
	for name, sources := range candidates {
		select {
		case <-ctx.Done():
			break runLoop
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(name string, sources map[string]bool) {
			defer wg.Done()
			defer func() { <-sem }()

			result := resolveOne(ctx, name, sources, timeout)

			j.mu.Lock()
			j.results = append(j.results, result)
			j.mu.Unlock()

			m.hub.Broadcast(entity.EventSubdomainResult, map[string]any{"scan_id": id, "result": result})
		}(name, sources)
	}
	wg.Wait()

	j.mu.Lock()
	if j.status == entity.ScanRunning {
		j.status = entity.ScanDone
	}
	status := j.status
	j.mu.Unlock()

	eventType := entity.EventSubdomainDone
	if status == entity.ScanStopped {
		eventType = entity.EventSubdomainStopped
	}
	m.hub.Broadcast(eventType, map[string]any{"scan_id": id})
}

func resolveOne(ctx context.Context, name string, sourceSet map[string]bool, timeout time.Duration) Result {
	started := time.Now()
	lookupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}

	ips, err := net.DefaultResolver.LookupHost(lookupCtx, name)
	durationMs := time.Since(started).Milliseconds()
	if err != nil {
		return Result{Subdomain: name, Resolved: false, Sources: sources, DurationMs: durationMs}
	}
	return Result{Subdomain: name, Resolved: true, IPs: ips, Sources: sources, DurationMs: durationMs}
}

func (m *Manager) Stop(id string) bool {
	m.mu.Lock()
	j, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return false
	}
	j.mu.Lock()
	if j.status == entity.ScanRunning {
		j.status = entity.ScanStopped
	}
	j.mu.Unlock()
	j.cancel()
	return true
}

func (m *Manager) Snapshot(id string) (status entity.ScanStatus, total int, results []Result, warning string, ok bool) {
	m.mu.Lock()
	j, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return "", 0, nil, "", false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	// A nil slice marshals to JSON `null`, not `[]` — always return a
	// non-nil slice so the frontend never has to guard against that.
	results = make([]Result, len(j.results))
	copy(results, j.results)
	return j.status, j.total, results, j.warning, true
}
