package xss

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nullsec/backend/internal/entity"
)

func TestClassify(t *testing.T) {
	payload := `<script>alert(1)</script>`

	t.Run("verbatim reflection is vulnerable", func(t *testing.T) {
		f := &Finding{}
		classify(f, "before "+payload+" after", payload)
		if !f.Vulnerable || f.Severity != entity.SeverityCritical {
			t.Fatalf("got Vulnerable=%v Severity=%v, want vulnerable/critical", f.Vulnerable, f.Severity)
		}
	})

	t.Run("escaped reflection is not vulnerable", func(t *testing.T) {
		f := &Finding{}
		classify(f, "before "+html.EscapeString(payload)+" after", payload)
		if f.Vulnerable {
			t.Fatalf("escaped reflection must not be flagged vulnerable, got Note=%q", f.Note)
		}
	})

	t.Run("no reflection at all", func(t *testing.T) {
		f := &Finding{}
		classify(f, "nothing interesting here", payload)
		if f.Vulnerable {
			t.Fatalf("no reflection must not be flagged vulnerable")
		}
	})
}

func TestParseFormBody(t *testing.T) {
	if _, ok := parseFormBody(""); ok {
		t.Fatal("empty body should report no params")
	}
	if _, ok := parseFormBody("   "); ok {
		t.Fatal("whitespace-only body should report no params")
	}
	values, ok := parseFormBody("a=1&b=2")
	if !ok || values.Get("a") != "1" || values.Get("b") != "2" {
		t.Fatalf("got %v, ok=%v", values, ok)
	}
}

// echoServer reflects the requested query param and posted form param
// verbatim (unescaped) in its response body, so Run() has a real vulnerable
// endpoint to find — this is the same shape a genuinely unsanitized app
// would have.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		_ = r.ParseForm()
		f := r.PostFormValue("f")
		fmt.Fprintf(w, "<html>query=%s form=%s</html>", q, f)
	}))
}

func TestRun_FindsQueryReflection(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	report := Run(context.Background(), Request{
		Method: http.MethodGet,
		URL:    srv.URL + "?q=seed",
	})

	if report.Error != "" {
		t.Fatalf("unexpected error: %s", report.Error)
	}
	if report.Summary.Vulnerable == 0 {
		t.Fatalf("expected at least one vulnerable finding for an echo server, got summary=%+v", report.Summary)
	}
	for _, f := range report.Findings {
		if f.Vulnerable && f.Location != "query" {
			t.Errorf("vulnerable finding on unexpected location %q", f.Location)
		}
	}
}

func TestRun_FindsBodyReflection(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	report := Run(context.Background(), Request{
		Method:  http.MethodPost,
		URL:     srv.URL,
		Headers: http.Header{"Content-Type": []string{"application/x-www-form-urlencoded"}},
		Body:    "f=seed",
	})

	if report.Error != "" {
		t.Fatalf("unexpected error: %s", report.Error)
	}
	if report.Summary.Vulnerable == 0 {
		t.Fatalf("expected at least one vulnerable finding for an echo server, got summary=%+v", report.Summary)
	}
}

func TestRun_NoFalsePositiveAgainstSanitizingServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		fmt.Fprintf(w, "<html>query=%s</html>", html.EscapeString(q))
	}))
	defer srv.Close()

	report := Run(context.Background(), Request{
		Method: http.MethodGet,
		URL:    srv.URL + "?q=seed",
	})

	if report.Summary.Vulnerable != 0 {
		t.Fatalf("a server that HTML-escapes its output must not be flagged vulnerable, got %+v", report.Summary)
	}
}

func TestRun_EnsuresSchemeOnBareHost(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	// Strip the "http://" that httptest.Server always includes, so this
	// exercises the same bare "host:port?query" shape a user pastes into
	// the UI. If urlnorm didn't run before url.Parse, Run would take its
	// early "invalid URL" return and never even get as far as discovering
	// the "q" query param, let alone attempting a request for it.
	bareTarget := strings.TrimPrefix(srv.URL, "http://") + "?q=seed"

	report := Run(context.Background(), Request{Method: http.MethodGet, URL: bareTarget})
	if report.Error != "" {
		t.Fatalf("got a URL-parse error, meaning the scheme was never added: %s", report.Error)
	}
	if report.Summary.ParamsTested != 1 {
		t.Fatalf("expected the \"q\" query param to be discovered and tested, got summary=%+v", report.Summary)
	}
}
