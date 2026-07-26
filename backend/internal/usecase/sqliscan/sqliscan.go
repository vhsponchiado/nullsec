// Package sqliscan batch-applies the SQL injection check (usecase/sqli)
// across every request already captured in Proxy History that matches a
// host filter, streaming one result per URL back over the WS hub as it
// completes — the same async job shape as portscan/subdomain/spider,
// applied to whatever traffic the user has already recorded instead of a
// single manually-entered request.
//
// This performs real requests against every matching target — only use it
// against hosts you're authorized to test.
package sqliscan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/usecase/sqli"
)

const (
	defaultConcurrency = 3
	maxConcurrency     = 10
	// perItemTimeout bounds a single URL's check — sqli.Run's time-based
	// probes deliberately delay the target, so a batch of many URLs could
	// otherwise stall indefinitely without a per-item cap.
	perItemTimeout = 2 * time.Minute
)

type StartRequest struct {
	// Host filters which captured requests to scan; empty scans all of
	// Proxy History.
	Host        string `json:"host"`
	Concurrency int    `json:"concurrency"`
}

// ItemResult is one scanned history entry's outcome.
type ItemResult struct {
	HistoryID string      `json:"history_id"`
	Method    string      `json:"method"`
	URL       string      `json:"url"`
	Report    sqli.Report `json:"report"`
}

type scan struct {
	mu      sync.Mutex
	status  entity.ScanStatus
	total   int
	results []ItemResult
	cancel  context.CancelFunc
}

type Manager struct {
	mu      sync.Mutex
	scans   map[string]*scan
	hub     entity.Broadcaster
	history entity.HistoryRepository
}

func NewManager(hub entity.Broadcaster, history entity.HistoryRepository) *Manager {
	return &Manager{scans: make(map[string]*scan), hub: hub, history: history}
}

// Start loads every matching history item, dedupes identical
// method+URL+body combinations, and launches the batch scan in the
// background, returning its id and item count immediately.
func (m *Manager) Start(req StartRequest) (string, int, error) {
	items, err := m.history.List(entity.RequestFilter{Host: strings.TrimSpace(req.Host)})
	if err != nil {
		return "", 0, err
	}
	items = dedupe(items)
	if len(items) == 0 {
		return "", 0, errors.New("no matching requests found in history to scan")
	}

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	if concurrency > maxConcurrency {
		concurrency = maxConcurrency
	}

	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	s := &scan{status: entity.ScanRunning, total: len(items), cancel: cancel}

	m.mu.Lock()
	m.scans[id] = s
	m.mu.Unlock()

	go m.run(ctx, id, s, items, concurrency)

	return id, len(items), nil
}

func (m *Manager) run(ctx context.Context, id string, s *scan, items []entity.Request, concurrency int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

runLoop:
	for _, item := range items {
		select {
		case <-ctx.Done():
			break runLoop
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(item entity.Request) {
			defer wg.Done()
			defer func() { <-sem }()

			itemCtx, cancel := context.WithTimeout(ctx, perItemTimeout)
			defer cancel()

			report := sqli.Run(itemCtx, sqli.Request{
				Method:  item.Method,
				URL:     item.URL,
				Headers: parseHeaders(item.Headers),
				Body:    item.Body,
			})
			result := ItemResult{HistoryID: item.ID, Method: item.Method, URL: item.URL, Report: report}

			s.mu.Lock()
			s.results = append(s.results, result)
			s.mu.Unlock()

			m.hub.Broadcast(entity.EventSQLiScanResult, map[string]any{"scan_id": id, "result": result})
		}(item)
	}
	wg.Wait()

	s.mu.Lock()
	if s.status == entity.ScanRunning {
		s.status = entity.ScanDone
	}
	status := s.status
	s.mu.Unlock()

	eventType := entity.EventSQLiScanDone
	if status == entity.ScanStopped {
		eventType = entity.EventSQLiScanStopped
	}
	m.hub.Broadcast(eventType, map[string]any{"scan_id": id})
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

func (m *Manager) Snapshot(id string) (status entity.ScanStatus, total int, results []ItemResult, ok bool) {
	m.mu.Lock()
	s, ok := m.scans[id]
	m.mu.Unlock()
	if !ok {
		return "", 0, nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	results = make([]ItemResult, len(s.results))
	copy(results, s.results)
	return s.status, s.total, results, true
}

// dedupe drops requests that repeat an earlier item's method+URL+body
// exactly, keeping the most recent occurrence (List returns newest first) —
// captured traffic often hits the same endpoint many times with identical
// parameters, and there's nothing new to learn from testing it twice.
func dedupe(items []entity.Request) []entity.Request {
	seen := make(map[string]bool, len(items))
	out := make([]entity.Request, 0, len(items))
	for _, it := range items {
		key := it.Method + " " + it.URL + "\x00" + it.Body
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, it)
	}
	return out
}

func parseHeaders(raw string) http.Header {
	var m map[string][]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
		return http.Header{}
	}
	return http.Header(m)
}
