// Package subdomain implements a subdomain finder combining these techniques:
//
//   - Passive (subdomains): queries crt.sh (Certificate Transparency logs)
//     for certificates issued for *.domain — reveals real subdomains without
//     ever touching the target.
//   - Passive (HackerTarget): queries HackerTarget's free hostsearch API,
//     which aggregates passive DNS data independent of crt.sh — a different
//     data source entirely, so it turns up names crt.sh doesn't have (and
//     keeps working when crt.sh itself is down, which is common — it's a
//     free, community-run service with no SLA).
//   - Passive (keyword/broad): queries crt.sh for any certificate whose name
//     contains a given keyword anywhere, with no requirement that it share
//     the target domain's DNS hierarchy. This surfaces "sibling" domains a
//     company registered under a similar-looking name (e.g. a QA/staging
//     environment at qa-example.com rather than qa.example.com) — a plain
//     subdomain search structurally cannot find these, since they aren't
//     subdomains at all, just similarly-named, unrelated apex domains.
//   - Active (subdomains): DNS brute-force, trying common subdomain prefixes
//     (from a SecLists wordlist, ranked by real-world frequency) and
//     resolving each candidate.
//   - Active (sibling domains): the same wordlist, but joined with a hyphen
//     instead of a dot (prefix + "-" + domain instead of prefix + "." +
//     domain) — an active counterpart to the keyword/broad crt.sh search
//     above, for organizations whose QA/staging/dev environment was never
//     issued a publicly-trusted certificate at all (so it can never show up
//     in Certificate Transparency, no matter how it's queried) but still
//     resolves in DNS.
//
// This performs real DNS lookups (and HTTPS requests to crt.sh) — only use
// it against domains you're authorized to test.
package subdomain

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// Source tags recorded on each Result, identifying how a candidate was
// found. Shared between the Go side and the frontend (see SOURCE_LABEL in
// subdomain-finder.tsx) — keep the two in sync if these change.
const (
	sourceRoot         = "root"
	sourceCRTSH        = "crtsh"
	sourceCRTSHKeyword = "crtsh-keyword"
	sourceHackerTarget = "hackertarget"
	sourceBruteforce   = "bruteforce"
	sourceSiblingBrute = "sibling-bruteforce"
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
// record for *.domain, returning the deduped, lowercased hostnames that are
// domain itself or a true subdomain of it. It retries on failure since a
// single crt.sh timeout/hiccup is common and otherwise silently drops every
// subdomain that was only ever discoverable through Certificate
// Transparency, not the brute-force wordlist.
func queryCRTSH(ctx context.Context, httpClients HTTPClientProvider, domain string) ([]string, error) {
	suffix := "." + domain
	keep := func(name string) bool { return name == domain || strings.HasSuffix(name, suffix) }
	return queryCRTSHWithRetry(ctx, httpClients, "%."+domain, keep)
}

// queryCRTSHKeyword fetches every certificate name (including SANs) crt.sh
// has on record containing keyword anywhere, with no requirement that the
// match share domain's DNS hierarchy — this is what surfaces a differently
// registered, similarly-named "sibling" domain (see the package doc
// comment). Every name that matches is kept, so callers should expect
// results with no structural relationship to the original target.
func queryCRTSHKeyword(ctx context.Context, httpClients HTTPClientProvider, keyword string) ([]string, error) {
	keep := func(string) bool { return true }
	return queryCRTSHWithRetry(ctx, httpClients, "%"+keyword+"%", keep)
}

// queryCRTSHWithRetry runs queryCRTSHOnce up to crtshRetries times, retrying
// on any transient failure (timeout, rate-limit, connection reset).
func queryCRTSHWithRetry(ctx context.Context, httpClients HTTPClientProvider, query string, keep func(string) bool) ([]string, error) {
	var lastErr error
	for attempt := 1; attempt <= crtshRetries; attempt++ {
		names, err := queryCRTSHOnce(ctx, httpClients, query, keep)
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

// queryCRTSHOnce runs query against crt.sh once and returns every deduped,
// lowercased certificate name for which keep returns true.
func queryCRTSHOnce(ctx context.Context, httpClients HTTPClientProvider, query string, keep func(string) bool) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, crtshTimeout)
	defer cancel()

	reqURL := fmt.Sprintf("https://crt.sh/?q=%s&output=json", query)
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
	for _, e := range entries {
		for _, line := range strings.Split(e.NameValue, "\n") {
			name := strings.ToLower(strings.TrimSpace(line))
			name = strings.TrimPrefix(name, "*.")
			if name == "" || !keep(name) || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}

const hackerTargetTimeout = 15 * time.Second

// hackerTargetRateLimited is the substring HackerTarget's free (no API key)
// tier responds with once the caller's daily quota is used up — the
// response is still HTTP 200 with a plain-text body, not an HTTP error
// status, so this has to be checked explicitly rather than relying on the
// status code.
const hackerTargetRateLimited = "API count exceeded"

// queryHackerTarget fetches every subdomain HackerTarget's free hostsearch
// API has on record for domain via its own passive DNS dataset — a
// different data source from crt.sh entirely, so it turns up names crt.sh
// doesn't have, and keeps working when crt.sh itself is down.
func queryHackerTarget(ctx context.Context, httpClients HTTPClientProvider, domain string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, hackerTargetTimeout)
	defer cancel()

	reqURL := "https://api.hackertarget.com/hostsearch/?q=" + domain
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClients.HTTPClient(hackerTargetTimeout).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hackertarget returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	text := string(body)
	if strings.Contains(text, hackerTargetRateLimited) {
		return nil, errors.New("hackertarget's free daily quota is used up for this IP")
	}

	suffix := "." + domain
	seen := make(map[string]bool)
	var out []string
	for _, line := range strings.Split(text, "\n") {
		host, _, _ := strings.Cut(strings.TrimSpace(line), ",")
		host = strings.ToLower(host)
		if host == "" || seen[host] {
			continue
		}
		if host != domain && !strings.HasSuffix(host, suffix) {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	return out, nil
}

type Result struct {
	Subdomain string `json:"subdomain"`
	// SameRoot is false for a keyword/broad-search hit that isn't actually
	// domain itself or a subdomain of it — just a similarly-named, distinct
	// apex domain. The frontend uses this to avoid implying a DNS
	// relationship that doesn't exist.
	SameRoot   bool     `json:"same_root"`
	Resolved   bool     `json:"resolved"`
	IPs        []string `json:"ips,omitempty"`
	Sources    []string `json:"sources"`
	DurationMs int64    `json:"duration_ms"`
}

type StartRequest struct {
	Domain   string `json:"domain"`
	UseCRTSH bool   `json:"use_crtsh"`
	// UseHackerTarget additionally queries HackerTarget's free hostsearch
	// API (see queryHackerTarget) — an independent passive DNS data source
	// from crt.sh, so it finds different names and keeps working when
	// crt.sh itself is down.
	UseHackerTarget bool `json:"use_hackertarget"`
	WordlistSize    int  `json:"wordlist_size"` // 0 disables brute-forcing
	// CRTSHKeyword, if non-empty, additionally runs a broad crt.sh search
	// for this keyword (see queryCRTSHKeyword) — independent of UseCRTSH
	// and WordlistSize, since it answers a different question ("what other
	// domains has this org registered that look related?") rather than
	// "what subdomains does this domain have?".
	CRTSHKeyword string `json:"crtsh_keyword"`
	// SiblingWordlistSize, like WordlistSize, brute-forces prefixes from
	// Wordlist against domain, but joined with "-" instead of "." — guessing
	// sibling apex domains (qa-domain.com) instead of subdomains
	// (qa.domain.com). 0 disables it.
	SiblingWordlistSize int `json:"sibling_wordlist_size"`
	Concurrency         int `json:"concurrency"`
	TimeoutMs           int `json:"timeout_ms"`
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
	keyword := strings.TrimSpace(req.CRTSHKeyword)
	if !req.UseCRTSH && !req.UseHackerTarget && req.WordlistSize <= 0 && keyword == "" && req.SiblingWordlistSize <= 0 {
		return "", 0, errors.New("enable crt.sh, HackerTarget, DNS brute-force, a keyword search, sibling-domain brute-force, or some combination")
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
	siblingWordlistSize := req.SiblingWordlistSize
	if siblingWordlistSize > len(Wordlist) {
		siblingWordlistSize = len(Wordlist)
	}

	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{status: entity.ScanRunning, cancel: cancel}

	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()

	go m.run(ctx, id, j, domain, req.UseCRTSH, req.UseHackerTarget, keyword, wordlistSize, siblingWordlistSize, timeout, concurrency)

	// The candidate count (and therefore total) isn't known until crt.sh
	// responds, which happens inside run(); the caller gets the eventual
	// total via the first snapshot/WS updates instead of blocking here.
	return id, wordlistSize + siblingWordlistSize + 1, nil
}

func (m *Manager) run(ctx context.Context, id string, j *job, domain string, useCRTSH, useHackerTarget bool, keyword string, wordlistSize, siblingWordlistSize int, timeout time.Duration, concurrency int) {
	candidates := make(map[string]map[string]bool)
	addCandidate := func(name, source string) {
		if candidates[name] == nil {
			candidates[name] = make(map[string]bool)
		}
		candidates[name][source] = true
	}

	addCandidate(domain, sourceRoot)

	var warnings []string
	addWarning := func(w string) {
		j.mu.Lock()
		warnings = append(warnings, w)
		j.warning = strings.Join(warnings, "; ")
		j.mu.Unlock()
	}

	if useCRTSH {
		names, err := queryCRTSH(ctx, m.httpClients, domain)
		switch {
		case err != nil:
			addWarning("crt.sh lookup failed: " + err.Error())
		case len(names) == 0:
			// crt.sh answered with no error but zero matching certificates.
			// That's sometimes a genuine result (a domain with no other
			// certs in Certificate Transparency logs), but it's also what a
			// silently-truncated or rate-limited response looks like — crt.sh
			// gives no way to tell those apart. Flag it so a run that looks
			// suspiciously thinner than a previous one isn't mistaken for
			// "these subdomains stopped existing."
			addWarning("crt.sh returned no certificates for this domain — this can be a genuine empty result, or crt.sh silently truncating/rate-limiting; re-run if you expected more here")
		default:
			for _, name := range names {
				addCandidate(name, sourceCRTSH)
			}
		}
	}

	if useHackerTarget {
		names, err := queryHackerTarget(ctx, m.httpClients, domain)
		switch {
		case err != nil:
			addWarning("HackerTarget lookup failed: " + err.Error())
		case len(names) == 0:
			addWarning("HackerTarget returned no subdomains for this domain — this can be a genuine empty result, or a quota/rate-limit issue; re-run if you expected more here")
		default:
			for _, name := range names {
				addCandidate(name, sourceHackerTarget)
			}
		}
	}

	if keyword != "" {
		names, err := queryCRTSHKeyword(ctx, m.httpClients, keyword)
		switch {
		case err != nil:
			addWarning("crt.sh keyword search failed: " + err.Error())
		case len(names) == 0:
			addWarning(fmt.Sprintf("crt.sh keyword search for %q returned no certificates — this can be genuine, or crt.sh silently truncating/rate-limiting; re-run if you expected more here", keyword))
		default:
			for _, name := range names {
				addCandidate(name, sourceCRTSHKeyword)
			}
		}
	}

	for i := 0; i < wordlistSize; i++ {
		addCandidate(Wordlist[i]+"."+domain, sourceBruteforce)
	}

	for i := 0; i < siblingWordlistSize; i++ {
		addCandidate(Wordlist[i]+"-"+domain, sourceSiblingBrute)
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

			result := resolveOne(ctx, name, domain, sources, timeout)

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

func resolveOne(ctx context.Context, name, domain string, sourceSet map[string]bool, timeout time.Duration) Result {
	started := time.Now()
	lookupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sameRoot := name == domain || strings.HasSuffix(name, "."+domain)

	ips, err := net.DefaultResolver.LookupHost(lookupCtx, name)
	durationMs := time.Since(started).Milliseconds()
	if err != nil {
		return Result{Subdomain: name, SameRoot: sameRoot, Resolved: false, Sources: sources, DurationMs: durationMs}
	}
	return Result{Subdomain: name, SameRoot: sameRoot, Resolved: true, IPs: ips, Sources: sources, DurationMs: durationMs}
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
