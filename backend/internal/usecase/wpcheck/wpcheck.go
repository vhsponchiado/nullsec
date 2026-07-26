// Package wpcheck implements a lightweight, read-only WordPress
// fingerprinting scan: it requests a fixed list of well-known WordPress
// paths against a target and reports which ones appear to exist, using
// simple per-path heuristics (status code, redirect target, body markers).
// It never authenticates, submits forms, or writes anything — purely GET
// requests for recon, the same kind of check tools like WPScan start with.
package wpcheck

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/urlnorm"
)

type check struct {
	Path        string
	Category    string
	Description string
}

var checks = []check{
	{"/wp-admin/", "Core / admin", "Admin panel"},
	{"/wp-login.php", "Core / admin", "Login screen (common brute-force target)"},
	{"/wp-cron.php", "Core / admin", "Runs scheduled tasks, can be abused for DoS"},
	{"/xmlrpc.php", "Core / admin", "Legacy API used by mobile apps/pingback; historically used for amplified brute-force and pingback DDoS amplification attacks"},

	{"/wp-json/", "REST API", "REST API root"},
	{"/wp-json/wp/v2/users", "REST API", "Often leaks usernames (user enumeration)"},
	{"/wp-json/wp/v2/posts", "REST API", "Content, usually harmless but can expose drafts/metadata"},
	{"/wp-json/wp/v2/pages", "REST API", "Content, usually harmless but can expose drafts/metadata"},
	{"/wp-json/wp/v2/media", "REST API", "Content, usually harmless but can expose drafts/metadata"},

	{"/?author=1", "User enumeration", "Redirects to /author/<username>/, revealing valid logins"},
	{"/?author=2", "User enumeration", "Redirects to /author/<username>/, revealing valid logins"},

	{"/wp-content/uploads/", "Static files & structure", "Media uploads, sometimes with forgotten sensitive files"},
	{"/wp-content/plugins/", "Static files & structure", "Allows fingerprinting installed plugins"},
	{"/wp-content/themes/", "Static files & structure", "Allows fingerprinting installed themes"},
	{"/wp-includes/", "Static files & structure", "Core files"},
	{"/readme.html", "Static files & structure", "Reveals the exact WordPress version"},
	{"/license.txt", "Static files & structure", "Default core license file"},
	{"/wp-config.php.bak", "Static files & structure", "Forgotten backup — CRITICAL if present"},
	{"/wp-config.php.save", "Static files & structure", "Forgotten backup — CRITICAL if present"},
	{"/wp-config.sql", "Static files & structure", "Forgotten DB dump — CRITICAL if present"},
	{"/wp-config.zip", "Static files & structure", "Forgotten backup — CRITICAL if present"},

	{"/feed/", "Other", "RSS, can leak authors/content"},
	{"/comments/feed/", "Other", "Comments RSS"},
	{"/wp-content/debug.log", "Other", "If WP_DEBUG_LOG is on and misconfigured, leaks stack traces — CRITICAL if present"},
	{"/wp-admin/admin-ajax.php", "Other", "Endpoint used by plugins, large attack surface depending on what's registered"},
}

type Result struct {
	Path        string          `json:"path"`
	URL         string          `json:"url"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	Status      int             `json:"status"`
	Found       bool            `json:"found"`
	Severity    entity.Severity `json:"severity"`
	Note        string          `json:"note"`
	Error       string          `json:"error,omitempty"`
}

type Summary struct {
	Checked  int `json:"checked"`
	Found    int `json:"found"`
	Critical int `json:"critical"`
}

var client = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

var versionRe = regexp.MustCompile(`[Vv]ersion\s+(\d+\.\d+(?:\.\d+)?)`)

// Run performs all checks against baseURL concurrently and returns the
// results in their canonical (declared) order. baseURL is assumed to be
// HTTPS if it has no explicit scheme.
func Run(ctx context.Context, baseURL string) ([]Result, Summary) {
	base := strings.TrimRight(urlnorm.EnsureScheme(baseURL), "/")
	results := make([]Result, len(checks))

	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		go func(i int, c check) {
			defer wg.Done()
			results[i] = runCheck(ctx, base, c)
		}(i, c)
	}
	wg.Wait()

	summary := Summary{Checked: len(results)}
	for _, r := range results {
		if r.Found {
			summary.Found++
		}
		if r.Severity == entity.SeverityCritical {
			summary.Critical++
		}
	}

	return results, summary
}

func runCheck(ctx context.Context, base string, c check) Result {
	url := base + c.Path
	res := Result{
		Path:        c.Path,
		URL:         url,
		Category:    c.Category,
		Description: c.Description,
		Severity:    entity.SeverityInfo,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NullsecWPCheck/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()

	res.Status = resp.StatusCode
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	body := string(bodyBytes)
	location := resp.Header.Get("Location")

	classify(&res, c, body, location)
	return res
}

func classify(res *Result, c check, body, location string) {
	status := res.Status

	switch c.Path {
	case "/wp-login.php":
		res.Found = status == 200 && (strings.Contains(body, "user_login") || strings.Contains(body, "wp-login"))
		if res.Found {
			res.Severity = entity.SeverityWarning
			res.Note = "WordPress login form found"
		}

	case "/xmlrpc.php":
		res.Found = status == 405 || strings.Contains(body, "XML-RPC server accepts POST requests only")
		if res.Found {
			res.Severity = entity.SeverityWarning
			res.Note = "xmlrpc.php is active — surface for amplified brute-force"
		}

	case "/wp-json/":
		res.Found = status == 200 && strings.Contains(body, "namespaces")
		if res.Found {
			res.Note = "REST API exposed"
		}

	case "/wp-json/wp/v2/users":
		res.Found = status == 200 && strings.Contains(body, "\"slug\"")
		if res.Found {
			res.Severity = entity.SeverityCritical
			res.Note = "User enumeration via REST API possible"
		}

	case "/wp-json/wp/v2/posts", "/wp-json/wp/v2/pages", "/wp-json/wp/v2/media":
		res.Found = status == 200 && strings.HasPrefix(strings.TrimSpace(body), "[")
		if res.Found {
			res.Note = "Endpoint accessible"
		}

	case "/?author=1", "/?author=2":
		res.Found = (status == 301 || status == 302) && strings.Contains(location, "/author/")
		if res.Found {
			res.Severity = entity.SeverityCritical
			res.Note = "Username revealed via redirect: " + location
		}

	case "/wp-admin/":
		res.Found = status == 200 || status == 302 || status == 401
		if res.Found {
			res.Note = "wp-admin responded (" + describeAdminStatus(status, location) + ")"
		}

	case "/readme.html":
		res.Found = status == 200 && strings.Contains(body, "WordPress")
		if res.Found {
			res.Severity = entity.SeverityWarning
			if m := versionRe.FindStringSubmatch(body); len(m) > 1 {
				res.Note = "Version exposed: " + m[1]
			} else {
				res.Note = "readme.html accessible"
			}
		}

	case "/wp-content/debug.log":
		res.Found = status == 200
		if res.Found {
			res.Severity = entity.SeverityCritical
			res.Note = "Debug log publicly exposed"
		}

	case "/wp-config.php.bak", "/wp-config.php.save", "/wp-config.sql", "/wp-config.zip":
		res.Found = status == 200
		if res.Found {
			res.Severity = entity.SeverityCritical
			res.Note = "Sensitive backup exposed"
		}

	case "/wp-content/uploads/", "/wp-content/plugins/", "/wp-content/themes/", "/wp-includes/":
		res.Found = status == 200 || status == 403
		if res.Found {
			res.Note = describeDirStatus(status)
		}

	case "/feed/", "/comments/feed/":
		res.Found = status == 200
		if res.Found {
			res.Note = "RSS feed accessible"
		}

	case "/wp-admin/admin-ajax.php":
		res.Found = status == 400 || status == 200
		if res.Found {
			res.Note = "admin-ajax.php responded"
		}

	default:
		res.Found = status == 200
	}
}

func describeAdminStatus(status int, location string) string {
	switch status {
	case 200:
		return "200, possible listing/direct access"
	case 302:
		if location != "" {
			return "redirected to " + location
		}
		return "redirected (likely login screen)"
	case 401:
		return "401, protected by HTTP auth"
	default:
		return "unexpected status"
	}
}

func describeDirStatus(status int) string {
	if status == 200 {
		return "directory listing open"
	}
	return "directory exists (listing blocked)"
}
