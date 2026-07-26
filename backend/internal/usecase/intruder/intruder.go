// Package intruder implements a Burp Intruder-equivalent attack engine:
// positions are marked in a request template with §…§, one or more payload
// sets are combined according to an attack type (sniper, battering ram,
// pitchfork, cluster bomb), and every resulting request is fired at the
// target with the results streamed back over the WS hub as they complete.
//
// This performs real requests against the target — only use it against
// hosts you're authorized to test.
package intruder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nullsec/backend/internal/entity"
)

// HTTPClientProvider hands back an *http.Client routed through whatever
// upstream proxy is currently configured. The only implementation is
// infra/upstream.Manager, but this use case doesn't know that.
type HTTPClientProvider interface {
	HTTPClient(timeout time.Duration) *http.Client
}

type AttackType string

const (
	Sniper       AttackType = "sniper"
	BatteringRam AttackType = "battering_ram"
	Pitchfork    AttackType = "pitchfork"
	ClusterBomb  AttackType = "cluster_bomb"
)

// MaxRequests caps how many requests a single attack may fire, checked
// before anything is sent so a mis-sized cluster bomb can't be launched
// accidentally.
const MaxRequests = 20000

// Concurrency and per-request timeout are both configurable per attack
// (StartRequest.Concurrency / TimeoutMs), clamped to these bounds. The
// default (5 concurrent, 15s) matches this engine's original fixed
// behavior; the max is deliberately far lower than, say, the port
// scanner's — every request here is a real HTTP request against a live
// endpoint (often something as sensitive as a login form), and going much
// faster risks tripping account lockouts, rate limits, or just overloading
// the target, not just "finishing sooner."
const (
	defaultConcurrency = 5
	maxConcurrency     = 50
	defaultTimeout     = 15 * time.Second
	minTimeoutMs       = 1000
	maxTimeoutMs       = 60000
	// maxRatePerSec caps StartRequest.RatePerSec. 0 (the default) means no
	// cap beyond Concurrency itself — this only matters when someone wants
	// to throttle *below* what Concurrency alone would allow, e.g. to stay
	// under a target's rate limit or WAF threshold.
	maxRatePerSec = 100
)

var markerRe = regexp.MustCompile(`§([^§]*)§`)

type HeaderPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Template is the base request with §…§ markers still embedded.
type Template struct {
	Method  string       `json:"method"`
	URL     string       `json:"url"`
	Headers []HeaderPair `json:"headers"`
	Body    string       `json:"body"`
}

// Positions extracts the base (unmarked) value of every §…§ marker in the
// template, in a fixed order: URL, then headers in row order, then body.
func Positions(t Template) []string {
	var bases []string
	bases = append(bases, extractBases(t.URL)...)
	for _, h := range t.Headers {
		bases = append(bases, extractBases(h.Value)...)
	}
	bases = append(bases, extractBases(t.Body)...)
	return bases
}

func extractBases(s string) []string {
	matches := markerRe.FindAllStringSubmatch(s, -1)
	bases := make([]string, len(matches))
	for i, m := range matches {
		bases[i] = m[1]
	}
	return bases
}

// build substitutes every marker in the template with the corresponding
// entry of values (same order Positions returns), producing a concrete,
// marker-free request.
func build(t Template, values []string) (method, url string, headers []HeaderPair, body string) {
	idx := 0
	replace := func(s string) string {
		return markerRe.ReplaceAllStringFunc(s, func(string) string {
			v := values[idx]
			idx++
			return v
		})
	}
	url = replace(t.URL)
	headers = make([]HeaderPair, len(t.Headers))
	for i, h := range t.Headers {
		headers[i] = HeaderPair{Key: h.Key, Value: replace(h.Value)}
	}
	body = replace(t.Body)
	return t.Method, url, headers, body
}

// EstimateTotal computes how many requests an attack would fire without
// generating them, so the caller can reject oversized attacks cheaply.
func EstimateTotal(attackType AttackType, numPositions int, sets [][]string) (int, error) {
	if numPositions == 0 {
		return 0, fmt.Errorf("no positions marked — wrap at least one value in §…§")
	}
	switch attackType {
	case Sniper:
		if len(sets) == 0 || len(sets[0]) == 0 {
			return 0, fmt.Errorf("payload set 1 is empty")
		}
		return numPositions * len(sets[0]), nil
	case BatteringRam:
		if len(sets) == 0 || len(sets[0]) == 0 {
			return 0, fmt.Errorf("payload set 1 is empty")
		}
		return len(sets[0]), nil
	case Pitchfork:
		if len(sets) != numPositions {
			return 0, fmt.Errorf("pitchfork needs exactly %d payload sets (one per position), got %d", numPositions, len(sets))
		}
		min := len(sets[0])
		for _, s := range sets[1:] {
			if len(s) < min {
				min = len(s)
			}
		}
		if min == 0 {
			return 0, fmt.Errorf("one of the payload sets is empty")
		}
		return min, nil
	case ClusterBomb:
		if len(sets) != numPositions {
			return 0, fmt.Errorf("cluster bomb needs exactly %d payload sets (one per position), got %d", numPositions, len(sets))
		}
		total := 1
		for _, s := range sets {
			if len(s) == 0 {
				return 0, fmt.Errorf("one of the payload sets is empty")
			}
			total *= len(s)
		}
		return total, nil
	default:
		return 0, fmt.Errorf("unknown attack type: %s", attackType)
	}
}

// plan generates the per-request replacement values (one row per request,
// one entry per position) for the given attack type.
func plan(attackType AttackType, bases []string, sets [][]string) ([][]string, error) {
	n := len(bases)
	total, err := EstimateTotal(attackType, n, sets)
	if err != nil {
		return nil, err
	}
	out := make([][]string, 0, total)

	switch attackType {
	case Sniper:
		set := sets[0]
		for posIdx := 0; posIdx < n; posIdx++ {
			for _, payload := range set {
				row := append([]string(nil), bases...)
				row[posIdx] = payload
				out = append(out, row)
			}
		}
	case BatteringRam:
		for _, payload := range sets[0] {
			row := make([]string, n)
			for i := range row {
				row[i] = payload
			}
			out = append(out, row)
		}
	case Pitchfork:
		for i := 0; i < total; i++ {
			row := make([]string, n)
			for p := 0; p < n; p++ {
				row[p] = sets[p][i]
			}
			out = append(out, row)
		}
	case ClusterBomb:
		idxs := make([]int, n)
		for {
			row := make([]string, n)
			for p := 0; p < n; p++ {
				row[p] = sets[p][idxs[p]]
			}
			out = append(out, row)

			p := n - 1
			for p >= 0 {
				idxs[p]++
				if idxs[p] < len(sets[p]) {
					break
				}
				idxs[p] = 0
				p--
			}
			if p < 0 {
				break
			}
		}
	}
	return out, nil
}

type ResultSummary struct {
	Index      int      `json:"index"`
	Payloads   []string `json:"payloads"`
	Status     int      `json:"status"`
	Length     int      `json:"length"`
	DurationMs int64    `json:"duration_ms"`
	Match      bool     `json:"match,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type ResultDetail struct {
	ResultSummary
	RequestMethod   string `json:"request_method"`
	RequestURL      string `json:"request_url"`
	RequestHeaders  string `json:"request_headers"`
	RequestBody     string `json:"request_body"`
	ResponseHeaders string `json:"response_headers"`
	ResponseBody    string `json:"response_body"`
}

type StartRequest struct {
	Template    Template   `json:"template"`
	AttackType  AttackType `json:"attack_type"`
	PayloadSets [][]string `json:"payload_sets"`
	Grep        string     `json:"grep"`
	// Concurrency is how many requests run at once; 0 uses defaultConcurrency.
	Concurrency int `json:"concurrency"`
	// TimeoutMs is the per-request timeout in milliseconds; 0 uses defaultTimeout.
	TimeoutMs int `json:"timeout_ms"`
	// RatePerSec caps how many requests are *dispatched* per second, on top
	// of Concurrency; 0 means unthrottled (only Concurrency limits things in
	// flight at once). Use this to stay under a target's rate limit or WAF
	// threshold even when Concurrency alone would go faster.
	RatePerSec int `json:"rate_per_sec"`
}

type attack struct {
	mu      sync.Mutex
	status  entity.ScanStatus
	total   int
	results []ResultDetail
	cancel  context.CancelFunc
}

// Wordlists are the built-in lists offered as Intruder payload sources —
// passwords, usernames, and a paired username:password credentials list
// (see CredentialUsernames/CredentialPasswords). The only implementation is
// infra/wordlist, but this use case doesn't know that.
type Wordlists interface {
	Passwords() []string
	Usernames() []string
	CredentialUsernames() []string
	CredentialPasswords() []string
}

type Manager struct {
	mu          sync.Mutex
	attacks     map[string]*attack
	hub         entity.Broadcaster
	httpClients HTTPClientProvider
	wordlists   Wordlists
}

func NewManager(hub entity.Broadcaster, httpClients HTTPClientProvider, wordlists Wordlists) *Manager {
	return &Manager{
		attacks:     make(map[string]*attack),
		hub:         hub,
		httpClients: httpClients,
		wordlists:   wordlists,
	}
}

// Passwords returns the built-in password wordlist.
func (m *Manager) Passwords() []string { return m.wordlists.Passwords() }

// Usernames returns the built-in username wordlist.
func (m *Manager) Usernames() []string { return m.wordlists.Usernames() }

// CredentialUsernames and CredentialPasswords return the two halves of the
// built-in username:password pairs, in matching order — see
// wordlist.CredentialUsernames's doc comment for how to use them together.
func (m *Manager) CredentialUsernames() []string { return m.wordlists.CredentialUsernames() }
func (m *Manager) CredentialPasswords() []string { return m.wordlists.CredentialPasswords() }

// Start validates and launches an attack, returning its id and total request
// count immediately; the requests themselves run in a background goroutine.
func (m *Manager) Start(req StartRequest) (string, int, error) {
	bases := Positions(req.Template)
	total, err := EstimateTotal(req.AttackType, len(bases), req.PayloadSets)
	if err != nil {
		return "", 0, err
	}
	if total > MaxRequests {
		return "", 0, fmt.Errorf("this attack would fire %d requests, which is over the %d limit — reduce the payload sets", total, MaxRequests)
	}

	rows, err := plan(req.AttackType, bases, req.PayloadSets)
	if err != nil {
		return "", 0, err
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

	ratePerSec := req.RatePerSec
	if ratePerSec < 0 {
		ratePerSec = 0
	}
	if ratePerSec > maxRatePerSec {
		ratePerSec = maxRatePerSec
	}

	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	a := &attack{status: entity.ScanRunning, total: len(rows), cancel: cancel}

	m.mu.Lock()
	m.attacks[id] = a
	m.mu.Unlock()

	go m.run(ctx, id, a, req.Template, rows, req.Grep, concurrency, timeout, ratePerSec)

	return id, len(rows), nil
}

func (m *Manager) run(ctx context.Context, id string, a *attack, tmpl Template, rows [][]string, grep string, concurrency int, timeout time.Duration, ratePerSec int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	// A ticker throttles how often new requests are *dispatched*, on top of
	// (not instead of) the concurrency semaphore above — this is what lets a
	// low RatePerSec stay under a target's rate limit even if Concurrency
	// alone would fire faster. The first request always goes out immediately.
	var throttle <-chan time.Time
	if ratePerSec > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(ratePerSec))
		defer ticker.Stop()
		throttle = ticker.C
	}

sendLoop:
	for i, values := range rows {
		select {
		case <-ctx.Done():
			break sendLoop
		default:
		}
		if throttle != nil && i > 0 {
			select {
			case <-throttle:
			case <-ctx.Done():
				break sendLoop
			}
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(index int, values []string) {
			defer wg.Done()
			defer func() { <-sem }()

			result := m.sendOne(ctx, tmpl, values, grep, timeout)
			result.Index = index

			a.mu.Lock()
			a.results = append(a.results, result)
			a.mu.Unlock()

			m.hub.Broadcast(entity.EventIntruderResult, map[string]any{
				"attack_id": id,
				"result":    result.ResultSummary,
			})
		}(i, values)
	}
	wg.Wait()

	a.mu.Lock()
	if a.status == entity.ScanRunning {
		a.status = entity.ScanDone
	}
	status := a.status
	a.mu.Unlock()

	eventType := entity.EventIntruderDone
	if status == entity.ScanStopped {
		eventType = entity.EventIntruderStopped
	}
	m.hub.Broadcast(eventType, map[string]any{"attack_id": id})
}

func (m *Manager) sendOne(ctx context.Context, tmpl Template, values []string, grep string, timeout time.Duration) ResultDetail {
	method, url, headers, body := build(tmpl, values)
	started := time.Now()

	outReq, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		return ResultDetail{ResultSummary: ResultSummary{Payloads: values, Error: err.Error()}}
	}
	for _, h := range headers {
		if strings.EqualFold(h.Key, "Host") {
			outReq.Host = h.Value
			continue
		}
		outReq.Header.Add(h.Key, h.Value)
	}

	reqHeadersJSON, _ := json.Marshal(outReq.Header)

	resp, err := m.httpClients.HTTPClient(timeout).Do(outReq)
	durationMs := time.Since(started).Milliseconds()
	if err != nil {
		return ResultDetail{
			ResultSummary:  ResultSummary{Payloads: values, DurationMs: durationMs, Error: err.Error()},
			RequestMethod:  method,
			RequestURL:     url,
			RequestHeaders: string(reqHeadersJSON),
			RequestBody:    body,
		}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	respHeadersJSON, _ := json.Marshal(resp.Header)

	match := grep != "" && bytes.Contains(respBody, []byte(grep))

	return ResultDetail{
		ResultSummary: ResultSummary{
			Payloads:   values,
			Status:     resp.StatusCode,
			Length:     len(respBody),
			DurationMs: durationMs,
			Match:      match,
		},
		RequestMethod:   method,
		RequestURL:      url,
		RequestHeaders:  string(reqHeadersJSON),
		RequestBody:     body,
		ResponseHeaders: string(respHeadersJSON),
		ResponseBody:    string(respBody),
	}
}

func (m *Manager) Stop(id string) bool {
	m.mu.Lock()
	a, ok := m.attacks[id]
	m.mu.Unlock()
	if !ok {
		return false
	}
	a.mu.Lock()
	if a.status == entity.ScanRunning {
		a.status = entity.ScanStopped
	}
	a.mu.Unlock()
	a.cancel()
	return true
}

// Snapshot returns the current status, total and all results collected so
// far — used both for the polling GET endpoint and to recover state after a
// WS reconnect.
func (m *Manager) Snapshot(id string) (status entity.ScanStatus, total int, results []ResultSummary, ok bool) {
	m.mu.Lock()
	a, ok := m.attacks[id]
	m.mu.Unlock()
	if !ok {
		return "", 0, nil, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	summaries := make([]ResultSummary, len(a.results))
	for i, r := range a.results {
		summaries[i] = r.ResultSummary
	}
	return a.status, a.total, summaries, true
}

func (m *Manager) Result(id string, index int) (ResultDetail, bool) {
	m.mu.Lock()
	a, ok := m.attacks[id]
	m.mu.Unlock()
	if !ok {
		return ResultDetail{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.results {
		if r.Index == index {
			return r, true
		}
	}
	return ResultDetail{}, false
}
