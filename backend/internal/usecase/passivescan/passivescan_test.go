package passivescan

import (
	"testing"

	"nullsec/backend/internal/entity"
)

func headersJSON(pairs map[string][]string) string {
	if len(pairs) == 0 {
		return "{}"
	}
	out := "{"
	first := true
	for k, vs := range pairs {
		if !first {
			out += ","
		}
		first = false
		out += `"` + k + `":[`
		for i, v := range vs {
			if i > 0 {
				out += ","
			}
			out += `"` + v + `"`
		}
		out += "]"
	}
	return out + "}"
}

func baseInput() Input {
	return Input{ID: "req-1", Method: "GET", URL: "https://example.com/", Host: "example.com"}
}

func TestCheckSecurityHeaders_AllMissing(t *testing.T) {
	in := baseInput()
	findings := checkSecurityHeaders(in, headersOf(nil), true)
	if len(findings) != 5 {
		t.Fatalf("got %d findings, want 5 (CSP, HSTS, nosniff, clickjacking, referrer-policy): %+v", len(findings), findings)
	}
}

func TestCheckSecurityHeaders_AllPresent(t *testing.T) {
	in := baseInput()
	h := headersOf(map[string][]string{
		"Content-Security-Policy":   {"default-src 'self'"},
		"Strict-Transport-Security": {"max-age=31536000"},
		"X-Content-Type-Options":    {"nosniff"},
		"X-Frame-Options":           {"DENY"},
		"Referrer-Policy":           {"no-referrer"},
	})
	if findings := checkSecurityHeaders(in, h, true); len(findings) != 0 {
		t.Fatalf("got %+v, want no findings when every header is present", findings)
	}
}

func TestCheckSecurityHeaders_HSTSOnlyMattersOverHTTPS(t *testing.T) {
	in := baseInput()
	h := headersOf(map[string][]string{
		"Content-Security-Policy": {"default-src 'self'"},
		"X-Content-Type-Options":  {"nosniff"},
		"X-Frame-Options":         {"DENY"},
		"Referrer-Policy":         {"no-referrer"},
	})
	if findings := checkSecurityHeaders(in, h, false); len(findings) != 0 {
		t.Fatalf("got %+v, want no HSTS finding over plain HTTP", findings)
	}
}

func TestCheckSecurityHeaders_CSPFrameAncestorsSatisfiesClickjacking(t *testing.T) {
	in := baseInput()
	h := headersOf(map[string][]string{
		"Content-Security-Policy": {"frame-ancestors 'none'"},
	})
	findings := checkSecurityHeaders(in, h, false)
	for _, f := range findings {
		if f.Title == "Missing clickjacking protection" {
			t.Fatal("a CSP frame-ancestors directive should satisfy the clickjacking check even with no X-Frame-Options")
		}
	}
}

func TestCheckCookies_MissingFlags(t *testing.T) {
	in := baseInput()
	h := headersOf(map[string][]string{"Set-Cookie": {"session=abc123"}})
	findings := checkCookies(in, h)
	if len(findings) != 1 {
		t.Fatalf("got %+v, want 1 finding", findings)
	}
	if findings[0].Severity != entity.SeverityWarning {
		t.Fatalf("severity = %v, want warning", findings[0].Severity)
	}
}

func TestCheckCookies_AllFlagsPresent(t *testing.T) {
	in := baseInput()
	h := headersOf(map[string][]string{"Set-Cookie": {"session=abc123; HttpOnly; Secure; SameSite=Strict"}})
	if findings := checkCookies(in, h); len(findings) != 0 {
		t.Fatalf("got %+v, want no findings", findings)
	}
}

func TestCheckCookies_MultipleCookiesEachChecked(t *testing.T) {
	in := baseInput()
	h := headersOf(map[string][]string{"Set-Cookie": {
		"a=1; HttpOnly; Secure; SameSite=Strict",
		"b=2", // missing everything
	}})
	findings := checkCookies(in, h)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want exactly 1 (only cookie b is missing flags): %+v", len(findings), findings)
	}
}

func TestCheckSensitiveParams(t *testing.T) {
	in := baseInput()
	in.URL = "https://example.com/login?password=hunter2&foo=bar"
	findings := checkSensitiveParams(in)
	if len(findings) != 1 {
		t.Fatalf("got %+v, want exactly 1 finding for the password param", findings)
	}
}

func TestCheckSensitiveParams_NoSensitiveNames(t *testing.T) {
	in := baseInput()
	in.URL = "https://example.com/search?q=hello&page=2"
	if findings := checkSensitiveParams(in); len(findings) != 0 {
		t.Fatalf("got %+v, want no findings", findings)
	}
}

func TestCheckInfoDisclosure(t *testing.T) {
	in := baseInput()
	h := headersOf(map[string][]string{"Server": {"nginx/1.18.0"}})
	findings := checkInfoDisclosure(in, h)
	if len(findings) != 1 {
		t.Fatalf("got %+v, want a version-disclosure finding", findings)
	}
}

func TestCheckInfoDisclosure_NoVersionNumber(t *testing.T) {
	in := baseInput()
	h := headersOf(map[string][]string{"Server": {"nginx"}})
	if findings := checkInfoDisclosure(in, h); len(findings) != 0 {
		t.Fatalf("got %+v, want no finding when no version number is present", findings)
	}
}

func TestCheckCORS(t *testing.T) {
	in := baseInput()

	t.Run("wildcard with credentials is critical", func(t *testing.T) {
		h := headersOf(map[string][]string{
			"Access-Control-Allow-Origin":      {"*"},
			"Access-Control-Allow-Credentials": {"true"},
		})
		findings := checkCORS(in, h)
		if len(findings) != 1 || findings[0].Severity != entity.SeverityCritical {
			t.Fatalf("got %+v, want 1 critical finding", findings)
		}
	})

	t.Run("wildcard alone is info", func(t *testing.T) {
		h := headersOf(map[string][]string{"Access-Control-Allow-Origin": {"*"}})
		findings := checkCORS(in, h)
		if len(findings) != 1 || findings[0].Severity != entity.SeverityInfo {
			t.Fatalf("got %+v, want 1 info finding", findings)
		}
	})

	t.Run("specific origin is not flagged", func(t *testing.T) {
		h := headersOf(map[string][]string{"Access-Control-Allow-Origin": {"https://trusted.example"}})
		if findings := checkCORS(in, h); len(findings) != 0 {
			t.Fatalf("got %+v, want no findings for a specific origin", findings)
		}
	})

	t.Run("no CORS header at all", func(t *testing.T) {
		if findings := checkCORS(in, headersOf(nil)); len(findings) != 0 {
			t.Fatalf("got %+v, want no findings", findings)
		}
	})
}

func TestCheckCleartextCredentials(t *testing.T) {
	t.Run("password over HTTP is critical", func(t *testing.T) {
		in := baseInput()
		in.URL = "http://example.com/login"
		in.RequestBody = "username=bob&password=hunter2"
		findings := checkCleartextCredentials(in, false)
		if len(findings) != 1 || findings[0].Severity != entity.SeverityCritical {
			t.Fatalf("got %+v, want 1 critical finding", findings)
		}
	})

	t.Run("password over HTTPS is not flagged", func(t *testing.T) {
		in := baseInput()
		in.RequestBody = "username=bob&password=hunter2"
		if findings := checkCleartextCredentials(in, true); len(findings) != 0 {
			t.Fatalf("got %+v, want no findings over HTTPS", findings)
		}
	})

	t.Run("no credentials over HTTP", func(t *testing.T) {
		in := baseInput()
		in.URL = "http://example.com/"
		in.RequestBody = "q=hello"
		if findings := checkCleartextCredentials(in, false); len(findings) != 0 {
			t.Fatalf("got %+v, want no findings", findings)
		}
	})
}

func TestRun_AggregatesSummaryCounts(t *testing.T) {
	inputs := []Input{
		{
			ID: "req-1", Method: "GET", URL: "http://example.com/login", Host: "example.com",
			RequestBody:     "password=hunter2", // critical: cleartext credentials
			ResponseHeaders: "{}",
		},
		{
			ID: "req-2", Method: "GET", URL: "https://example.com/", Host: "example.com",
			ResponseHeaders: headersJSON(map[string][]string{
				"Content-Security-Policy":   {"default-src 'self'"},
				"Strict-Transport-Security": {"max-age=1"},
				"X-Content-Type-Options":    {"nosniff"},
				"X-Frame-Options":           {"DENY"},
				"Referrer-Policy":           {"no-referrer"},
			}), // clean: no findings
		},
	}

	report := Run(inputs)
	if report.Summary.RequestsScanned != 2 {
		t.Fatalf("RequestsScanned = %d, want 2", report.Summary.RequestsScanned)
	}
	if report.Summary.Critical != 1 {
		t.Fatalf("Critical = %d, want 1, findings=%+v", report.Summary.Critical, report.Findings)
	}
	if report.Summary.Findings != len(report.Findings) {
		t.Fatalf("Summary.Findings (%d) should match len(Findings) (%d)", report.Summary.Findings, len(report.Findings))
	}
}

func TestRun_EmptyInput(t *testing.T) {
	report := Run(nil)
	if report.Summary.RequestsScanned != 0 || report.Summary.Findings != 0 {
		t.Fatalf("report = %+v, want all zero for no input", report)
	}
	if report.Findings == nil {
		t.Fatal("Findings should be an empty slice, not nil (so it JSON-encodes as [] rather than null)")
	}
}

// headersOf builds an http.Header-shaped value the same way parseHeaders
// would after decoding a captured entry's JSON-encoded response headers.
func headersOf(pairs map[string][]string) map[string][]string {
	if pairs == nil {
		return map[string][]string{}
	}
	return pairs
}
