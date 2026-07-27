// Package dirfinder implements a gobuster-style content discovery scanner:
// for each candidate path from a wordlist (optionally combined with a set
// of file extensions), it requests target+"/"+candidate and reports what
// came back — status code, size, and redirect target — the same technique
// gobuster's "dir" mode and OWASP DirBuster use to surface directories and
// files a site never links to.
//
// This makes real requests against the target — only use it against hosts
// you're authorized to test.
package dirfinder

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/urlnorm"
)

// HTTPClientProvider hands back an *http.Client routed through whatever
// upstream proxy is currently configured. The only implementation is
// infra/upstream.Manager, but this use case doesn't know that.
type HTTPClientProvider interface {
	HTTPClient(timeout time.Duration) *http.Client
}

//go:embed wordlist.txt
var rawWordlist string

// Wordlist is the built-in list of common web content paths (SecLists'
// Discovery/Web-Content/common.txt), alphabetically ordered.
var Wordlist = strings.Split(strings.TrimSpace(rawWordlist), "\n")

const (
	// MaxCandidates caps wordlist size * (1 + len(extensions)) so a large
	// wordlist combined with several extensions can't silently turn into a
	// six-figure request count.
	MaxCandidates      = 50000
	defaultConcurrency = 20
	maxConcurrency     = 100
	defaultTimeout     = 5 * time.Second
	minTimeoutMs       = 200
	maxTimeoutMs       = 20000
	// bodyPeekLimit bounds how much of the body gets read to compute a size
	// when the server didn't send Content-Length — just enough to report a
	// meaningful number, not enough to matter for a 50k-request scan.
	bodyPeekLimit = 4096
)

type Result struct {
	Path       string `json:"path"`
	URL        string `json:"url"`
	Status     int    `json:"status"`
	Size       int64  `json:"size"`
	Redirect   string `json:"redirect,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

type StartRequest struct {
	Target       string   `json:"target"`
	WordlistSize int      `json:"wordlist_size"`
	Extensions   []string `json:"extensions"`
	Concurrency  int      `json:"concurrency"`
	TimeoutMs    int      `json:"timeout_ms"`
}

type job struct {
	mu      sync.Mutex
	status  entity.ScanStatus
	total   int
	results []Result
	cancel  context.CancelFunc
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

// candidatePaths expands the first wordlistSize words into request paths,
// appending each requested extension to every word in addition to the bare
// word itself (mirroring gobuster's -x flag), and dedups the result — two
// extensions normalized to the same suffix (".php" and "php") shouldn't
// double a candidate's cost.
func candidatePaths(wordlistSize int, extensions []string) []string {
	if wordlistSize > len(Wordlist) {
		wordlistSize = len(Wordlist)
	}
	words := Wordlist[:wordlistSize]

	seen := make(map[string]bool, len(words))
	out := make([]string, 0, len(words))
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	for _, w := range words {
		add(w)
		for _, ext := range extensions {
			add(w + "." + ext)
		}
	}
	return out
}

func (m *Manager) Start(req StartRequest) (string, int, error) {
	target := urlnorm.EnsureScheme(req.Target)
	if target == "" {
		return "", 0, errors.New("target is required")
	}
	if req.WordlistSize <= 0 {
		return "", 0, errors.New("wordlist_size must be greater than zero")
	}
	base := strings.TrimRight(target, "/")

	extensions := make([]string, 0, len(req.Extensions))
	for _, e := range req.Extensions {
		e = strings.TrimPrefix(strings.TrimSpace(e), ".")
		if e != "" {
			extensions = append(extensions, e)
		}
	}

	paths := candidatePaths(req.WordlistSize, extensions)
	if len(paths) > MaxCandidates {
		return "", 0, fmt.Errorf("too many candidates (%d) — reduce the wordlist size or number of extensions (max %d)", len(paths), MaxCandidates)
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

	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{status: entity.ScanRunning, total: len(paths), cancel: cancel}

	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()

	go m.run(ctx, id, j, base, paths, timeout, concurrency)

	return id, len(paths), nil
}

func (m *Manager) run(ctx context.Context, id string, j *job, base string, paths []string, timeout time.Duration, concurrency int) {
	client := m.httpClients.HTTPClient(timeout)
	// Report the redirect itself instead of silently following it — a 301
	// to /admin/login is as interesting a finding as a 200, and following
	// it would hide which candidate path actually produced it.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

runLoop:
	for _, path := range paths {
		select {
		case <-ctx.Done():
			break runLoop
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(path string) {
			defer wg.Done()
			defer func() { <-sem }()

			result := probe(ctx, client, base, path, timeout)

			j.mu.Lock()
			j.results = append(j.results, result)
			j.mu.Unlock()

			m.hub.Broadcast(entity.EventDirFinderResult, map[string]any{"scan_id": id, "result": result})
		}(path)
	}
	wg.Wait()

	j.mu.Lock()
	if j.status == entity.ScanRunning {
		j.status = entity.ScanDone
	}
	status := j.status
	j.mu.Unlock()

	eventType := entity.EventDirFinderDone
	if status == entity.ScanStopped {
		eventType = entity.EventDirFinderStopped
	}
	m.hub.Broadcast(eventType, map[string]any{"scan_id": id})
}

func probe(ctx context.Context, client *http.Client, base, path string, timeout time.Duration) Result {
	target := base + "/" + path
	started := time.Now()

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return Result{Path: path, URL: target, Error: err.Error(), DurationMs: time.Since(started).Milliseconds()}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NullsecDirFinder/1.0)")

	resp, err := client.Do(req)
	durationMs := time.Since(started).Milliseconds()
	if err != nil {
		return Result{Path: path, URL: target, Error: err.Error(), DurationMs: durationMs}
	}
	defer resp.Body.Close()

	size := resp.ContentLength
	if size < 0 {
		n, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, bodyPeekLimit))
		size = n
	}

	return Result{
		Path:       path,
		URL:        target,
		Status:     resp.StatusCode,
		Size:       size,
		Redirect:   resp.Header.Get("Location"),
		DurationMs: durationMs,
	}
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

func (m *Manager) Snapshot(id string) (status entity.ScanStatus, total int, results []Result, ok bool) {
	m.mu.Lock()
	j, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return "", 0, nil, false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	// A nil slice marshals to JSON `null`, not `[]` — always return a
	// non-nil slice so the frontend never has to guard against that.
	results = make([]Result, len(j.results))
	copy(results, j.results)
	return j.status, j.total, results, true
}
