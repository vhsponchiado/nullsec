// Package passivescan analyzes traffic that's already been captured (in
// Proxy History) for common issues — missing security headers, poorly
// flagged cookies, sensitive-looking data in URLs, server version
// disclosure, permissive CORS, and credentials sent over plain HTTP.
//
// Unlike the other scanners in this app, this makes no requests of its
// own: it's a pure read of data already sitting in the database, so it's
// always safe to run regardless of authorization scope (though acting on
// what it finds is a different story).
package passivescan

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"nullsec/backend/internal/entity"
)

// Input mirrors the fields of a captured history entry this package needs.
// Kept as its own type (rather than depending on the store package) so this
// package stays a pure function of plain data.
type Input struct {
	ID              string
	Method          string
	URL             string
	Host            string
	RequestHeaders  string // JSON-encoded http.Header
	RequestBody     string
	Status          int
	ResponseHeaders string // JSON-encoded http.Header
	ResponseBody    string
}

type Finding struct {
	HistoryID string          `json:"history_id"`
	Method    string          `json:"method"`
	URL       string          `json:"url"`
	Host      string          `json:"host"`
	Category  string          `json:"category"`
	Title     string          `json:"title"`
	Severity  entity.Severity `json:"severity"`
	Detail    string          `json:"detail"`
}

type Summary struct {
	RequestsScanned int `json:"requests_scanned"`
	Findings        int `json:"findings"`
	Critical        int `json:"critical"`
	Warning         int `json:"warning"`
}

type Report struct {
	Findings []Finding `json:"findings"`
	Summary  Summary   `json:"summary"`
}

// Run analyzes every input and returns every finding across all of them.
func Run(inputs []Input) Report {
	findings := []Finding{}
	for _, in := range inputs {
		respHeaders := parseHeaders(in.ResponseHeaders)
		isHTTPS := strings.HasPrefix(strings.ToLower(in.URL), "https://")

		findings = append(findings, checkSecurityHeaders(in, respHeaders, isHTTPS)...)
		findings = append(findings, checkCookies(in, respHeaders)...)
		findings = append(findings, checkSensitiveParams(in)...)
		findings = append(findings, checkInfoDisclosure(in, respHeaders)...)
		findings = append(findings, checkCORS(in, respHeaders)...)
		findings = append(findings, checkCleartextCredentials(in, isHTTPS)...)
	}

	summary := Summary{RequestsScanned: len(inputs), Findings: len(findings)}
	for _, f := range findings {
		switch f.Severity {
		case entity.SeverityCritical:
			summary.Critical++
		case entity.SeverityWarning:
			summary.Warning++
		}
	}

	return Report{Findings: findings, Summary: summary}
}

func parseHeaders(raw string) http.Header {
	var m map[string][]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
		return http.Header{}
	}
	return http.Header(m)
}

func finding(in Input, category, title string, sev entity.Severity, detail string) Finding {
	return Finding{
		HistoryID: in.ID,
		Method:    in.Method,
		URL:       in.URL,
		Host:      in.Host,
		Category:  category,
		Title:     title,
		Severity:  sev,
		Detail:    detail,
	}
}

func checkSecurityHeaders(in Input, h http.Header, isHTTPS bool) []Finding {
	var out []Finding

	if h.Get("Content-Security-Policy") == "" {
		out = append(out, finding(in, "missing_header", "Missing Content-Security-Policy", entity.SeverityWarning,
			"No CSP header — the response has no baseline defense against XSS/data injection via script, style or frame restrictions."))
	}
	if isHTTPS && h.Get("Strict-Transport-Security") == "" {
		out = append(out, finding(in, "missing_header", "Missing Strict-Transport-Security", entity.SeverityWarning,
			"Served over HTTPS but no HSTS header — a network attacker could still downgrade a future visit to plain HTTP."))
	}
	if v := h.Get("X-Content-Type-Options"); !strings.EqualFold(v, "nosniff") {
		out = append(out, finding(in, "missing_header", "Missing X-Content-Type-Options", entity.SeverityInfo,
			"No 'nosniff' — browsers may MIME-sniff the response and execute it as a different content type than intended."))
	}
	if h.Get("X-Frame-Options") == "" && !strings.Contains(strings.ToLower(h.Get("Content-Security-Policy")), "frame-ancestors") {
		out = append(out, finding(in, "missing_header", "Missing clickjacking protection", entity.SeverityWarning,
			"No X-Frame-Options and no CSP frame-ancestors directive — the page can be framed by any site."))
	}
	if h.Get("Referrer-Policy") == "" {
		out = append(out, finding(in, "missing_header", "Missing Referrer-Policy", entity.SeverityInfo,
			"No Referrer-Policy — full URLs (possibly with sensitive query params) may leak to third parties via the Referer header on outbound links."))
	}

	return out
}

func checkCookies(in Input, h http.Header) []Finding {
	var out []Finding
	for _, raw := range h.Values("Set-Cookie") {
		name := raw
		if i := strings.Index(raw, "="); i >= 0 {
			name = raw[:i]
		}
		lower := strings.ToLower(raw)

		var missing []string
		if !strings.Contains(lower, "httponly") {
			missing = append(missing, "HttpOnly")
		}
		if !strings.Contains(lower, "secure") {
			missing = append(missing, "Secure")
		}
		if !strings.Contains(lower, "samesite") {
			missing = append(missing, "SameSite")
		}
		if len(missing) == 0 {
			continue
		}

		out = append(out, finding(in, "cookie",
			fmt.Sprintf("Cookie %q missing %s", strings.TrimSpace(name), strings.Join(missing, ", ")),
			entity.SeverityWarning,
			"Cookies without these flags are more exposed to theft via XSS (HttpOnly), network sniffing (Secure), or cross-site request forgery (SameSite)."))
	}
	return out
}

var sensitiveParamNames = []string{
	"password", "passwd", "pwd", "token", "api_key", "apikey", "secret",
	"session", "sessionid", "auth", "access_token", "refresh_token",
	"ssn", "credit_card", "card_number",
}

func checkSensitiveParams(in Input) []Finding {
	u, err := url.Parse(in.URL)
	if err != nil {
		return nil
	}
	var out []Finding
	for name := range u.Query() {
		lower := strings.ToLower(name)
		for _, s := range sensitiveParamNames {
			if strings.Contains(lower, s) {
				out = append(out, finding(in, "sensitive_param",
					fmt.Sprintf("Sensitive-looking parameter %q in the URL", name),
					entity.SeverityWarning,
					"Query string values get logged by servers/proxies, cached, stored in browser history, and leaked via the Referer header — sensitive data belongs in the body or a header, not the URL."))
				break
			}
		}
	}
	return out
}

var versionRe = regexp.MustCompile(`[0-9]+\.[0-9]+`)

func checkInfoDisclosure(in Input, h http.Header) []Finding {
	var out []Finding
	for _, key := range []string{"Server", "X-Powered-By"} {
		v := h.Get(key)
		if v != "" && versionRe.MatchString(v) {
			out = append(out, finding(in, "info_disclosure",
				fmt.Sprintf("%s header reveals version info", key),
				entity.SeverityInfo,
				fmt.Sprintf("%s: %s — lets an attacker target known vulnerabilities for that exact version.", key, v)))
		}
	}
	return out
}

func checkCORS(in Input, h http.Header) []Finding {
	origin := h.Get("Access-Control-Allow-Origin")
	if origin == "" {
		return nil
	}
	creds := strings.EqualFold(h.Get("Access-Control-Allow-Credentials"), "true")

	if origin == "*" && creds {
		return []Finding{finding(in, "cors", "CORS: wildcard origin with credentials allowed", entity.SeverityCritical,
			"Access-Control-Allow-Origin: * combined with Access-Control-Allow-Credentials: true. Most browsers reject this exact combination, but some servers instead reflect the request's Origin header verbatim while still allowing credentials — which IS exploitable. Worth confirming manually with a cross-origin request.")}
	}
	if origin == "*" {
		return []Finding{finding(in, "cors", "CORS: wildcard origin allowed", entity.SeverityInfo,
			"Access-Control-Allow-Origin: * — fine for a public, unauthenticated API; a problem if this endpoint also returns sensitive or session-specific data.")}
	}
	return nil
}

func checkCleartextCredentials(in Input, isHTTPS bool) []Finding {
	if isHTTPS {
		return nil
	}
	lower := strings.ToLower(in.URL + " " + in.RequestBody)
	for _, s := range []string{"password", "passwd", "pwd"} {
		if strings.Contains(lower, s) {
			return []Finding{finding(in, "cleartext", "Credentials sent over plain HTTP", entity.SeverityCritical,
				"This request looks like it carries a password/credential field over an unencrypted connection — trivially interceptable by anyone on the network path.")}
		}
	}
	return nil
}
