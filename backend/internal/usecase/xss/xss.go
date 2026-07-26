// Package xss implements a lightweight reflected-XSS signal scanner:
// given a request (method, URL, headers, body), it enumerates injectable
// parameters (query string + form-urlencoded body) and probes each one with
// a small set of well-known payloads, checking whether the payload survives
// unescaped in the response body — the core signal of a reflected XSS.
//
// A payload that comes back HTML-escaped (e.g. `&lt;script&gt;`) means the
// input reached the output but was sanitized here; that's recorded but not
// flagged as vulnerable. Only a verbatim, unescaped reflection is flagged —
// comparing against "does the raw payload appear at all" would flood
// results with sanitized-and-therefore-safe reflections.
//
// This performs real requests against the target — only use it against
// hosts you're authorized to test.
package xss

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/urlnorm"
)

type Request struct {
	Method  string
	URL     string
	Headers http.Header
	Body    string
}

type Finding struct {
	Parameter  string          `json:"parameter"`
	Location   string          `json:"location"` // "query" or "body"
	Payload    string          `json:"payload"`
	Context    Context         `json:"context"`
	Status     int             `json:"status"`
	ContentLen int             `json:"content_length"`
	Vulnerable bool            `json:"vulnerable"`
	Severity   entity.Severity `json:"severity"`
	Note       string          `json:"note"`
	Error      string          `json:"error,omitempty"`
}

type Summary struct {
	ParamsTested int `json:"params_tested"`
	Requests     int `json:"requests"`
	Vulnerable   int `json:"vulnerable"`
}

type Report struct {
	Findings []Finding `json:"findings"`
	Summary  Summary   `json:"summary"`
	Error    string    `json:"error,omitempty"`
}

var client = &http.Client{Timeout: 12 * time.Second}

const maxConcurrent = 6

type rawResponse struct {
	status int
	body   string
	err    error
}

type injectionPoint struct {
	name     string
	location string
}

// Run scans req for reflected-XSS signals and returns a report. req.URL is
// assumed to be HTTPS if it has no explicit scheme.
func Run(ctx context.Context, req Request) Report {
	req.URL = urlnorm.EnsureScheme(req.URL)
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return Report{Findings: []Finding{}, Error: "invalid URL: " + err.Error()}
	}

	queryParams := parsedURL.Query()
	bodyParams, hasBodyParams := parseFormBody(req.Body)

	var points []injectionPoint
	for name := range queryParams {
		points = append(points, injectionPoint{name, "query"})
	}
	if hasBodyParams {
		for name := range bodyParams {
			points = append(points, injectionPoint{name, "body"})
		}
	}

	sem := make(chan struct{}, maxConcurrent)
	send := func(p injectionPoint, payloadValue string) rawResponse {
		sem <- struct{}{}
		defer func() { <-sem }()

		var (
			reqURL  = req.URL
			reqBody = req.Body
		)
		if p.location == "query" {
			q := cloneValues(queryParams)
			q.Set(p.name, payloadValue)
			u := *parsedURL
			u.RawQuery = q.Encode()
			reqURL = u.String()
		} else {
			b := cloneValues(bodyParams)
			b.Set(p.name, payloadValue)
			reqBody = b.Encode()
		}
		return doRequest(ctx, req.Method, reqURL, req.Headers, reqBody)
	}

	var (
		mu       sync.Mutex
		findings = []Finding{}
		wg       sync.WaitGroup
	)
	for _, p := range points {
		for _, payload := range Payloads {
			wg.Add(1)
			go func(p injectionPoint, payload Payload) {
				defer wg.Done()
				raw := send(p, payload.Value)

				f := Finding{
					Parameter: p.name,
					Location:  p.location,
					Payload:   payload.Value,
					Context:   payload.Context,
					Status:    raw.status,
					Severity:  entity.SeverityInfo,
				}
				if raw.err != nil {
					f.Error = raw.err.Error()
				} else {
					f.ContentLen = len(raw.body)
					classify(&f, raw.body, payload.Value)
				}

				mu.Lock()
				findings = append(findings, f)
				mu.Unlock()
			}(p, payload)
		}
	}
	wg.Wait()

	summary := Summary{ParamsTested: len(points), Requests: len(findings)}
	for _, f := range findings {
		if f.Vulnerable {
			summary.Vulnerable++
		}
	}

	return Report{Findings: findings, Summary: summary}
}

// classify flags a finding as vulnerable only when the exact payload
// (special characters intact) shows up verbatim in the response — a
// self-contained, reliable signal that the input wasn't sanitized.
func classify(f *Finding, body, payload string) {
	escaped := html.EscapeString(payload)
	metaCharsPresent := escaped != payload

	if strings.Contains(body, payload) {
		if !metaCharsPresent {
			// This payload (e.g. a bare `javascript:` URI) has no HTML
			// metacharacters, so html.EscapeString is a no-op on it —
			// finding it verbatim proves nothing about whether the app
			// escapes HTML at all, only that this exact substring is
			// present. Flag it, but don't claim certain exploitability the
			// way a genuinely-unescaped `<script>` reflection would.
			f.Severity = entity.SeverityInfo
			f.Note = "Reflected verbatim, but this payload has no HTML metacharacters to escape — " +
				"reflection alone doesn't confirm it lands in an exploitable sink (e.g. an href)."
			return
		}
		f.Vulnerable = true
		f.Severity = entity.SeverityCritical
		f.Note = "Payload reflected unescaped in the response — likely exploitable XSS."
		return
	}

	if metaCharsPresent && strings.Contains(body, escaped) {
		f.Note = "Reflected, but HTML-escaped here — input reaches the output but looks sanitized."
		return
	}

	f.Note = "Not reflected in the response."
}

func doRequest(ctx context.Context, method, targetURL string, headers http.Header, body string) rawResponse {
	req, err := http.NewRequestWithContext(ctx, method, targetURL, strings.NewReader(body))
	if err != nil {
		return rawResponse{err: err}
	}
	if headers != nil {
		req.Header = headers.Clone()
	}

	resp, err := client.Do(req)
	if err != nil {
		return rawResponse{err: err}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	return rawResponse{
		status: resp.StatusCode,
		body:   string(bodyBytes),
	}
}

func parseFormBody(body string) (url.Values, bool) {
	if strings.TrimSpace(body) == "" {
		return nil, false
	}
	values, err := url.ParseQuery(body)
	if err != nil || len(values) == 0 {
		return nil, false
	}
	return values, true
}

func cloneValues(v url.Values) url.Values {
	clone := make(url.Values, len(v))
	for k, vv := range v {
		clone[k] = append([]string(nil), vv...)
	}
	return clone
}
