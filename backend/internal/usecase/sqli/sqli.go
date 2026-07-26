// Package sqli implements a lightweight SQL injection signal scanner:
// given a request (method, URL, headers, body), it enumerates injectable
// parameters (query string + form-urlencoded body) and probes each one for
// three independent signals:
//
//   - error-based: a payload triggers a recognizable DB error in the body.
//   - time-based: a payload measurably delays the response vs baseline.
//   - boolean-based: a "always true" and an "always false" condition are
//     each injected and their responses compared to EACH OTHER (not to the
//     baseline). If the app evaluates the injected SQL, true/false diverge;
//     if it's just rejecting malformed input, true and false get the exact
//     same (non-)response and no signal is raised.
//
// Comparing against the original baseline alone (as an earlier version of
// this scanner did) is unreliable: strict input validation rejects almost
// any injected payload with the same error, regardless of whether the
// underlying query is vulnerable, which floods results with false positives.
//
// This performs real requests against the target — only use it against
// hosts you're authorized to test.
package sqli

import (
	"context"
	"fmt"
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
	Parameter   string          `json:"parameter"`
	Location    string          `json:"location"` // "query" or "body"
	Payload     string          `json:"payload"`
	PayloadType PayloadType     `json:"payload_type"`
	Status      int             `json:"status"`
	DurationMs  int64           `json:"duration_ms"`
	ContentLen  int             `json:"content_length"`
	Vulnerable  bool            `json:"vulnerable"`
	Severity    entity.Severity `json:"severity"`
	Note        string          `json:"note"`
	Error       string          `json:"error,omitempty"`
}

type BooleanPairResult struct {
	Parameter    string          `json:"parameter"`
	Location     string          `json:"location"`
	TruePayload  string          `json:"true_payload"`
	FalsePayload string          `json:"false_payload"`
	TrueStatus   int             `json:"true_status"`
	FalseStatus  int             `json:"false_status"`
	TrueLen      int             `json:"true_content_length"`
	FalseLen     int             `json:"false_content_length"`
	Vulnerable   bool            `json:"vulnerable"`
	Severity     entity.Severity `json:"severity"`
	Note         string          `json:"note"`
	Error        string          `json:"error,omitempty"`
}

type Baseline struct {
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	ContentLen int    `json:"content_length"`
	Error      string `json:"error,omitempty"`
}

type Summary struct {
	ParamsTested int `json:"params_tested"`
	Requests     int `json:"requests"`
	Vulnerable   int `json:"vulnerable"`
	Critical     int `json:"critical"`
}

type Report struct {
	Baseline     Baseline            `json:"baseline"`
	Findings     []Finding           `json:"findings"`
	BooleanPairs []BooleanPairResult `json:"boolean_pairs"`
	Summary      Summary             `json:"summary"`
}

var client = &http.Client{
	Timeout: 12 * time.Second,
}

const maxConcurrent = 6

// timeBasedThresholdMs is how much slower than baseline (and in absolute
// terms) a time-based payload's response must be to count as a delay signal.
const timeBasedThresholdMs = 2500

type rawResponse struct {
	status     int
	durationMs int64
	body       string
	err        error
}

type injectionPoint struct {
	name     string
	location string
}

// Run scans req for SQL injection signals and returns a report. req.URL is
// assumed to be HTTPS if it has no explicit scheme.
func Run(ctx context.Context, req Request) Report {
	req.URL = urlnorm.EnsureScheme(req.URL)
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return Report{Baseline: Baseline{Error: "invalid URL: " + err.Error()}}
	}

	queryParams := parsedURL.Query()
	bodyParams, hasBodyParams := parseFormBody(req.Body)

	baselineRaw := doRequest(ctx, req.Method, req.URL, req.Headers, req.Body)
	baseline := Baseline{Status: baselineRaw.status, DurationMs: baselineRaw.durationMs, ContentLen: len(baselineRaw.body)}
	if baselineRaw.err != nil {
		baseline.Error = baselineRaw.err.Error()
	}

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
					Parameter:   p.name,
					Location:    p.location,
					Payload:     payload.Value,
					PayloadType: payload.Type,
					Status:      raw.status,
					DurationMs:  raw.durationMs,
					ContentLen:  len(raw.body),
					Severity:    entity.SeverityInfo,
				}
				if raw.err != nil {
					f.Error = raw.err.Error()
				} else {
					classify(&f, raw.body, baseline)
				}

				mu.Lock()
				findings = append(findings, f)
				mu.Unlock()
			}(p, payload)
		}
	}
	wg.Wait()

	var (
		pairMu sync.Mutex
		pairs  = []BooleanPairResult{}
		pairWg sync.WaitGroup
	)
	for _, p := range points {
		for _, bp := range BooleanPairs {
			pairWg.Add(1)
			go func(p injectionPoint, bp BooleanPair) {
				defer pairWg.Done()

				trueRaw := send(p, bp.True.Value)
				falseRaw := send(p, bp.False.Value)

				result := BooleanPairResult{
					Parameter:    p.name,
					Location:     p.location,
					TruePayload:  bp.True.Value,
					FalsePayload: bp.False.Value,
					TrueStatus:   trueRaw.status,
					FalseStatus:  falseRaw.status,
					TrueLen:      len(trueRaw.body),
					FalseLen:     len(falseRaw.body),
					Severity:     entity.SeverityInfo,
				}
				if trueRaw.err != nil {
					result.Error = trueRaw.err.Error()
				} else if falseRaw.err != nil {
					result.Error = falseRaw.err.Error()
				} else {
					classifyBooleanPair(&result, trueRaw, falseRaw)
				}

				pairMu.Lock()
				pairs = append(pairs, result)
				pairMu.Unlock()
			}(p, bp)
		}
	}
	pairWg.Wait()

	summary := Summary{ParamsTested: len(points), Requests: len(findings) + len(pairs)*2 + 1}
	for _, f := range findings {
		if f.Vulnerable {
			summary.Vulnerable++
		}
		if f.Severity == entity.SeverityCritical {
			summary.Critical++
		}
	}
	for _, p := range pairs {
		if p.Vulnerable {
			summary.Vulnerable++
		}
		if p.Severity == entity.SeverityCritical {
			summary.Critical++
		}
	}

	return Report{Baseline: baseline, Findings: findings, BooleanPairs: pairs, Summary: summary}
}

// classify only flags a standalone (non-paired) finding as vulnerable when
// there's a strong, self-contained signal: a DB error signature, or a
// measurable delay for time-based payloads. A status/length change alone is
// recorded for context but never marks the finding as vulnerable — that
// comparison is unreliable in isolation (see BooleanPairs for the reliable
// version of this check).
func classify(f *Finding, body string, baseline Baseline) {
	if db, ok := hasErrorSignature(body); ok {
		f.Vulnerable = true
		f.Severity = entity.SeverityCritical
		f.Note = fmt.Sprintf("Database error detected (%s) — likely SQL injection", db)
		return
	}

	if f.PayloadType == PayloadTime && f.DurationMs >= timeBasedThresholdMs &&
		f.DurationMs >= baseline.DurationMs+timeBasedThresholdMs {
		f.Vulnerable = true
		f.Severity = entity.SeverityCritical
		f.Note = fmt.Sprintf("~%.1fs delay vs %.1fs baseline — possible time-based SQLi", float64(f.DurationMs)/1000, float64(baseline.DurationMs)/1000)
		return
	}

	if baseline.Status != 0 && f.Status != baseline.Status {
		f.Note = fmt.Sprintf("Status changed from %d (baseline) to %d — isolated, not reliable on its own (see the paired boolean test)", baseline.Status, f.Status)
		return
	}

	f.Note = "No clear signal"
}

// classifyBooleanPair is the reliable boolean-blind check: it compares the
// TRUE and FALSE condition responses to each other. Divergence means the
// backend evaluated the injected condition (real signal); identical
// responses mean both were rejected the same way (just input validation).
func classifyBooleanPair(result *BooleanPairResult, trueRaw, falseRaw rawResponse) {
	statusDiffers := trueRaw.status != falseRaw.status

	lenDiff := len(trueRaw.body) - len(falseRaw.body)
	if lenDiff < 0 {
		lenDiff = -lenDiff
	}
	threshold := len(falseRaw.body) / 5 // 20%
	if threshold < 30 {
		threshold = 30
	}
	lenDiffers := lenDiff > threshold

	if statusDiffers || lenDiffers {
		result.Vulnerable = true
		result.Severity = entity.SeverityCritical
		result.Note = "The response changes depending on whether the condition is true or false — strong indicator of SQL injection."
		return
	}

	result.Note = "The application responded the same way to both conditions — no indication of SQL injection here."
}

func doRequest(ctx context.Context, method, targetURL string, headers http.Header, body string) rawResponse {
	started := time.Now()

	req, err := http.NewRequestWithContext(ctx, method, targetURL, strings.NewReader(body))
	if err != nil {
		return rawResponse{err: err}
	}
	if headers != nil {
		req.Header = headers.Clone()
	}

	resp, err := client.Do(req)
	if err != nil {
		return rawResponse{durationMs: time.Since(started).Milliseconds(), err: err}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	return rawResponse{
		status:     resp.StatusCode,
		durationMs: time.Since(started).Milliseconds(),
		body:       string(bodyBytes),
	}
}

func hasErrorSignature(body string) (string, bool) {
	lower := strings.ToLower(body)
	for _, sig := range errorSignatures {
		for _, marker := range sig.Markers {
			if strings.Contains(lower, marker) {
				return sig.Database, true
			}
		}
	}
	return "", false
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
