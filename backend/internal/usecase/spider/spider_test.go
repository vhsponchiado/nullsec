package spider

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"nullsec/backend/internal/entity"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestParseHTML_TitleAndLinks(t *testing.T) {
	base := mustParseURL(t, "https://example.com/")
	body := []byte(`
		<html><head><title>  My Page  </title></head>
		<body>
			<a href="/about">About</a>
			<a href="/about">About again (duplicate)</a>
			<a href="https://other.example/page">Cross-site</a>
			<a href="#fragment-only">Fragment</a>
			<a href="javascript:alert(1)">JS</a>
			<a href="mailto:test@example.com">Mail</a>
			<a href="tel:+15551234">Tel</a>
		</body></html>`)

	title, links, _ := parseHTML(body, base)
	if title != "My Page" {
		t.Fatalf("title = %q, want %q", title, "My Page")
	}

	want := map[string]bool{
		"https://example.com/about":  true,
		"https://other.example/page": true,
	}
	if len(links) != len(want) {
		t.Fatalf("links = %v, want exactly %d entries (duplicates and non-navigable hrefs excluded)", links, len(want))
	}
	for _, l := range links {
		if !want[l.String()] {
			t.Fatalf("unexpected link %q", l.String())
		}
	}
}

func TestParseHTML_Forms(t *testing.T) {
	base := mustParseURL(t, "https://example.com/")
	body := []byte(`
		<form action="/login" method="post">
			<input name="username">
			<input name="password">
		</form>`)

	_, _, forms := parseHTML(body, base)
	if len(forms) != 1 {
		t.Fatalf("forms = %+v, want exactly 1", forms)
	}
	f := forms[0]
	if f.Method != "POST" || f.Action != "https://example.com/login" {
		t.Fatalf("form = %+v", f)
	}
	if len(f.Inputs) != 2 || f.Inputs[0] != "username" || f.Inputs[1] != "password" {
		t.Fatalf("form inputs = %v", f.Inputs)
	}
}

func TestParseHTML_FormDefaultsToGETWithoutMethod(t *testing.T) {
	base := mustParseURL(t, "https://example.com/")
	body := []byte(`<form action="/search"><input name="q"></form>`)

	_, _, forms := parseHTML(body, base)
	if len(forms) != 1 || forms[0].Method != "GET" {
		t.Fatalf("forms = %+v, want method defaulting to GET", forms)
	}
}

func TestResolveLink(t *testing.T) {
	base := mustParseURL(t, "https://example.com/dir/page.html")

	cases := []struct {
		href string
		want string // "" means nil
	}{
		{"/about", "https://example.com/about"},
		{"relative.html", "https://example.com/dir/relative.html"},
		{"https://other.example/x", "https://other.example/x"},
		{"", ""},
		{"#top", ""},
		{"javascript:void(0)", ""},
		{"mailto:a@b.com", ""},
		{"tel:12345", ""},
		{"ftp://example.com/file", ""},
	}
	for _, c := range cases {
		t.Run(c.href, func(t *testing.T) {
			got := resolveLink(base, c.href)
			if c.want == "" {
				if got != nil {
					t.Fatalf("resolveLink(%q) = %v, want nil", c.href, got)
				}
				return
			}
			if got == nil || got.String() != c.want {
				t.Fatalf("resolveLink(%q) = %v, want %q", c.href, got, c.want)
			}
		})
	}
}

func TestInScope(t *testing.T) {
	seed := mustParseURL(t, "https://example.com/")

	cases := []struct {
		name              string
		target            string
		includeSubdomains bool
		want              bool
	}{
		{"same host", "https://example.com/page", false, true},
		{"subdomain excluded by default", "https://sub.example.com/page", false, false},
		{"subdomain included when allowed", "https://sub.example.com/page", true, true},
		{"unrelated host", "https://evil.com/page", false, false},
		{"unrelated host even with subdomains allowed", "https://evil.com/page", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target := mustParseURL(t, c.target)
			if got := inScope(target, seed, c.includeSubdomains); got != c.want {
				t.Fatalf("inScope(%q, includeSubdomains=%v) = %v, want %v", c.target, c.includeSubdomains, got, c.want)
			}
		})
	}
}

// fakeHistory is a minimal in-memory entity.HistoryRepository — only Insert
// is exercised by the crawler.
type fakeHistory struct {
	mu    sync.Mutex
	items []entity.Request
}

func (f *fakeHistory) Insert(r *entity.Request) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = append(f.items, *r)
	return nil
}
func (f *fakeHistory) Get(string) (*entity.Request, error)                 { return nil, nil }
func (f *fakeHistory) List(entity.RequestFilter) ([]entity.Request, error) { return nil, nil }
func (f *fakeHistory) ListSummary(entity.RequestFilter) ([]entity.RequestSummary, error) {
	return nil, nil
}
func (f *fakeHistory) Count(entity.RequestFilter) (int, error) { return 0, nil }
func (f *fakeHistory) Clear() error                            { return nil }
func (f *fakeHistory) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.items)
}

type fakeBroadcaster struct{}

func (fakeBroadcaster) Broadcast(string, any) {}

type directHTTPClientProvider struct{}

func (directHTTPClientProvider) HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func TestManager_CrawlFollowsLinksWithinScopeAndRespectsMaxPages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Home</title></head><body><a href="/a">A</a><a href="/b">B</a></body></html>`))
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>A</title></head><body>leaf</body></html>`))
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>B</title></head><body>leaf</body></html>`))
	})
	target := httptest.NewServer(mux)
	defer target.Close()

	history := &fakeHistory{}
	mgr := NewManager(fakeBroadcaster{}, history, directHTTPClientProvider{})

	id, err := mgr.Start(StartRequest{URL: target.URL, MaxDepth: 2, MaxPages: 10, Concurrency: 2})
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var pages []Page
	for time.Now().Before(deadline) {
		status, p, ok := mgr.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot returned ok=false for a just-started job")
		}
		if status == entity.ScanDone {
			pages = p
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pages == nil {
		t.Fatal("crawl never reached status=done")
	}
	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3 (home + 2 linked pages)", len(pages))
	}
	if history.count() != 3 {
		t.Fatalf("history recorded %d entries, want 3", history.count())
	}
}

func TestManager_CrawlRespectsMaxPagesCap(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/a">A</a><a href="/b">B</a><a href="/c">C</a></body></html>`))
	})
	leaf := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>leaf</body></html>`))
	}
	mux.HandleFunc("/a", leaf)
	mux.HandleFunc("/b", leaf)
	mux.HandleFunc("/c", leaf)
	target := httptest.NewServer(mux)
	defer target.Close()

	history := &fakeHistory{}
	mgr := NewManager(fakeBroadcaster{}, history, directHTTPClientProvider{})

	id, err := mgr.Start(StartRequest{URL: target.URL, MaxDepth: 2, MaxPages: 2, Concurrency: 1})
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, pages, ok := mgr.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot ok=false")
		}
		if status == entity.ScanDone {
			if len(pages) > 2 {
				t.Fatalf("got %d pages, want at most 2 (max_pages cap)", len(pages))
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("crawl never reached status=done")
}
