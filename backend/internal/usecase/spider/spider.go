// Package spider implements a web crawler ("spider" in ZAP/Burp terms):
// starting from a seed URL, it follows same-scope links, discovers pages
// and forms, and records every page it fetches into the same proxy history
// the MITM proxy itself writes to — crawled pages show up in Proxy History
// live, exactly like intercepted traffic, ready to send to Repeater/Intruder.
//
// This deliberately does NOT respect robots.txt: that file is a convention
// for search-engine crawlers, not a security boundary, and a pentest spider
// exists specifically to find what a polite crawler wouldn't. It does still
// bound itself with a max depth, a max page count, and a concurrency cap so
// it can't run away or hammer the target.
//
// This makes real requests against the target — only use it against hosts
// you're authorized to test.
package spider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/html"

	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/urlnorm"
)

// HTTPClientProvider hands back an *http.Client routed through whatever
// upstream proxy is currently configured. The only implementation is
// infra/upstream.Manager, but this use case doesn't know that.
type HTTPClientProvider interface {
	HTTPClient(timeout time.Duration) *http.Client
}

type Form struct {
	Action string   `json:"action"`
	Method string   `json:"method"`
	Inputs []string `json:"inputs"`
}

type Page struct {
	URL         string `json:"url"`
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Title       string `json:"title,omitempty"`
	Size        int    `json:"size"`
	DurationMs  int64  `json:"duration_ms"`
	Depth       int    `json:"depth"`
	LinksFound  int    `json:"links_found"`
	Forms       []Form `json:"forms,omitempty"`
	Error       string `json:"error,omitempty"`
}

type StartRequest struct {
	URL               string `json:"url"`
	MaxDepth          int    `json:"max_depth"`
	MaxPages          int    `json:"max_pages"`
	IncludeSubdomains bool   `json:"include_subdomains"`
	Concurrency       int    `json:"concurrency"`
}

const (
	defaultMaxDepth = 3
	maxMaxDepth     = 10
	defaultMaxPages = 200
	maxMaxPages     = 2000

	defaultConcurrency = 5
	maxConcurrency     = 20

	requestTimeout  = 10 * time.Second
	maxBodyReadSize = 2 << 20 // 2MB
	userAgent       = "nullsec-spider/1.0"
)

type job struct {
	mu     sync.Mutex
	status entity.ScanStatus
	pages  []Page
	cancel context.CancelFunc
}

type Manager struct {
	mu          sync.Mutex
	jobs        map[string]*job
	hub         entity.Broadcaster
	history     entity.HistoryRepository
	httpClients HTTPClientProvider
}

func NewManager(hub entity.Broadcaster, history entity.HistoryRepository, httpClients HTTPClientProvider) *Manager {
	return &Manager{jobs: make(map[string]*job), hub: hub, history: history, httpClients: httpClients}
}

func clamp(v, min, max, def int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	if v < min {
		return min
	}
	return v
}

func (m *Manager) Start(req StartRequest) (string, error) {
	raw := urlnorm.EnsureScheme(req.URL)
	seed, err := url.Parse(raw)
	if err != nil || seed.Host == "" {
		return "", errors.New("invalid URL")
	}

	maxDepth := clamp(req.MaxDepth, 1, maxMaxDepth, defaultMaxDepth)
	maxPages := clamp(req.MaxPages, 1, maxMaxPages, defaultMaxPages)
	concurrency := clamp(req.Concurrency, 1, maxConcurrency, defaultConcurrency)

	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{status: entity.ScanRunning, cancel: cancel}

	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()

	go m.crawl(ctx, id, j, seed, maxDepth, maxPages, concurrency, req.IncludeSubdomains)

	return id, nil
}

func (m *Manager) crawl(ctx context.Context, id string, j *job, seed *url.URL, maxDepth, maxPages, concurrency int, includeSubdomains bool) {
	type item struct {
		u     *url.URL
		depth int
	}

	var (
		mu        sync.Mutex
		visited   = map[string]bool{seed.String(): true}
		pageCount int
		wg        sync.WaitGroup
	)
	sem := make(chan struct{}, concurrency)

	var visit func(it item)
	visit = func(it item) {
		defer wg.Done()

		sem <- struct{}{}
		defer func() { <-sem }()

		select {
		case <-ctx.Done():
			return
		default:
		}

		mu.Lock()
		if pageCount >= maxPages {
			mu.Unlock()
			return
		}
		pageCount++
		mu.Unlock()

		page, links := m.fetchAndParse(ctx, it.u, it.depth)

		j.mu.Lock()
		j.pages = append(j.pages, page)
		j.mu.Unlock()
		m.hub.Broadcast(entity.EventSpiderPage, map[string]any{"spider_id": id, "page": page})

		if it.depth >= maxDepth {
			return
		}
		for _, link := range links {
			if !inScope(link, seed, includeSubdomains) {
				continue
			}
			key := link.String()

			mu.Lock()
			full := pageCount >= maxPages
			already := visited[key]
			if !already && !full {
				visited[key] = true
			}
			mu.Unlock()

			if already || full {
				continue
			}
			wg.Add(1)
			go visit(item{link, it.depth + 1})
		}
	}

	wg.Add(1)
	go visit(item{seed, 0})
	wg.Wait()

	j.mu.Lock()
	if j.status == entity.ScanRunning {
		j.status = entity.ScanDone
	}
	status := j.status
	j.mu.Unlock()

	eventType := entity.EventSpiderDone
	if status == entity.ScanStopped {
		eventType = entity.EventSpiderStopped
	}
	m.hub.Broadcast(eventType, map[string]any{"spider_id": id})
}

func (m *Manager) fetchAndParse(ctx context.Context, target *url.URL, depth int) (Page, []*url.URL) {
	started := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Page{URL: target.String(), Depth: depth, Error: err.Error()}, nil
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := m.httpClients.HTTPClient(requestTimeout).Do(req)
	durationMs := time.Since(started).Milliseconds()
	if err != nil {
		return Page{URL: target.String(), Depth: depth, DurationMs: durationMs, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyReadSize))
	contentType := resp.Header.Get("Content-Type")

	page := Page{
		URL:         target.String(),
		Status:      resp.StatusCode,
		ContentType: contentType,
		Size:        len(bodyBytes),
		DurationMs:  durationMs,
		Depth:       depth,
	}

	var links []*url.URL
	if strings.Contains(contentType, "text/html") {
		title, hrefs, forms := parseHTML(bodyBytes, target)
		page.Title = title
		page.Forms = forms
		page.LinksFound = len(hrefs)
		links = hrefs
	}

	m.saveToHistory(target, req.Header, resp.StatusCode, resp.Header, bodyBytes, durationMs)

	return page, links
}

// saveToHistory records the crawled page in the same proxy history the MITM
// proxy itself writes to, and broadcasts the same event it does — the
// Proxy History tab picks up crawled pages live, with no spider-specific
// frontend wiring needed for that part.
func (m *Manager) saveToHistory(target *url.URL, reqHeaders http.Header, status int, respHeaders http.Header, respBody []byte, durationMs int64) {
	reqHeadersJSON, _ := json.Marshal(reqHeaders)
	respHeadersJSON, _ := json.Marshal(respHeaders)

	rec := &entity.Request{
		ID:          uuid.NewString(),
		Method:      http.MethodGet,
		URL:         target.String(),
		Host:        target.Host,
		Headers:     string(reqHeadersJSON),
		Body:        "",
		Status:      status,
		RespHeaders: string(respHeadersJSON),
		RespBody:    string(respBody),
		Size:        len(respBody),
		DurationMs:  durationMs,
		Outcome:     "completed",
		CreatedAt:   time.Now(),
	}
	if err := m.history.Insert(rec); err != nil {
		return
	}
	// See proxy.Forwarder.saveRecord — broadcast only the lightweight
	// summary, not the full record with its (possibly large) bodies.
	m.hub.Broadcast(entity.EventRequestCompleted, rec.Summary())
}

// parseHTML walks the document once, collecting the title, every in-scope
// navigable link, and every form (action/method/input names) it finds.
func parseHTML(body []byte, base *url.URL) (title string, links []*url.URL, forms []Form) {
	seenLinks := make(map[string]bool)
	var currentForm *Form

	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}

		tok := z.Token()
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			switch tok.Data {
			case "title":
				if tt == html.StartTagToken && z.Next() == html.TextToken {
					title = strings.TrimSpace(z.Token().Data)
				}
			case "a":
				if href, ok := attr(tok, "href"); ok {
					if u := resolveLink(base, href); u != nil {
						key := u.String()
						if !seenLinks[key] {
							seenLinks[key] = true
							links = append(links, u)
						}
					}
				}
			case "form":
				f := Form{Method: "GET", Action: base.String()}
				if action, ok := attr(tok, "action"); ok {
					if u := resolveLink(base, action); u != nil {
						f.Action = u.String()
					}
				}
				if method, ok := attr(tok, "method"); ok {
					f.Method = strings.ToUpper(method)
				}
				currentForm = &f
			case "input", "select", "textarea":
				if currentForm != nil {
					if name, ok := attr(tok, "name"); ok && name != "" {
						currentForm.Inputs = append(currentForm.Inputs, name)
					}
				}
			}
		case html.EndTagToken:
			if tok.Data == "form" && currentForm != nil {
				forms = append(forms, *currentForm)
				currentForm = nil
			}
		}
	}
	return title, links, forms
}

func attr(tok html.Token, key string) (string, bool) {
	for _, a := range tok.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// resolveLink resolves href against base and filters out anything that
// isn't a navigable http(s) page (mailto:, tel:, javascript:, bare fragments).
func resolveLink(base *url.URL, href string) *url.URL {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return nil
	}
	lower := strings.ToLower(href)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tel:") {
		return nil
	}

	u, err := url.Parse(href)
	if err != nil {
		return nil
	}
	resolved := base.ResolveReference(u)
	resolved.Fragment = ""
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return nil
	}
	return resolved
}

func inScope(u, seed *url.URL, includeSubdomains bool) bool {
	if u.Hostname() == seed.Hostname() {
		return true
	}
	if includeSubdomains {
		return strings.HasSuffix(u.Hostname(), "."+seed.Hostname())
	}
	return false
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

func (m *Manager) Snapshot(id string) (status entity.ScanStatus, pages []Page, ok bool) {
	m.mu.Lock()
	j, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return "", nil, false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	pages = make([]Page, len(j.pages))
	copy(pages, j.pages)
	return j.status, pages, true
}
