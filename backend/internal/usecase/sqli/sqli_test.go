package sqli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nullsec/backend/internal/entity"
)

func TestHasErrorSignature(t *testing.T) {
	if _, ok := hasErrorSignature("everything is fine"); ok {
		t.Fatal("plain body should not match any error signature")
	}
	db, ok := hasErrorSignature("Warning: mysql_fetch_array() expects parameter 1 to be resource")
	if !ok || db == "" {
		t.Fatalf("expected a MySQL error signature match, got ok=%v db=%q", ok, db)
	}
}

func TestClassify(t *testing.T) {
	t.Run("DB error signature is vulnerable/critical", func(t *testing.T) {
		f := &Finding{}
		classify(f, "You have an error in your SQL syntax", Baseline{})
		if !f.Vulnerable || f.Severity != entity.SeverityCritical {
			t.Fatalf("got Vulnerable=%v Severity=%v", f.Vulnerable, f.Severity)
		}
	})

	t.Run("time-based payload past threshold is vulnerable/critical", func(t *testing.T) {
		f := &Finding{PayloadType: PayloadTime, DurationMs: 3000}
		classify(f, "ordinary response body", Baseline{DurationMs: 100})
		if !f.Vulnerable || f.Severity != entity.SeverityCritical {
			t.Fatalf("got Vulnerable=%v Severity=%v", f.Vulnerable, f.Severity)
		}
	})

	t.Run("time-based payload within threshold is not flagged", func(t *testing.T) {
		f := &Finding{PayloadType: PayloadTime, DurationMs: 300}
		classify(f, "ordinary response body", Baseline{DurationMs: 100})
		if f.Vulnerable {
			t.Fatalf("a 200ms delta should not cross the multi-second threshold")
		}
	})

	t.Run("status change alone is noted but not vulnerable", func(t *testing.T) {
		f := &Finding{Status: 500}
		classify(f, "ordinary response body", Baseline{Status: 200})
		if f.Vulnerable {
			t.Fatalf("an isolated status change must not be flagged vulnerable on its own")
		}
	})

	t.Run("nothing unusual", func(t *testing.T) {
		f := &Finding{Status: 200}
		classify(f, "ordinary response body", Baseline{Status: 200})
		if f.Vulnerable || f.Note != "No clear signal" {
			t.Fatalf("got Vulnerable=%v Note=%q", f.Vulnerable, f.Note)
		}
	})
}

func TestClassifyBooleanPair(t *testing.T) {
	t.Run("divergent responses are vulnerable", func(t *testing.T) {
		result := &BooleanPairResult{}
		trueRaw := rawResponse{status: 200, body: strings.Repeat("a", 500)}
		falseRaw := rawResponse{status: 200, body: strings.Repeat("a", 10)}
		classifyBooleanPair(result, trueRaw, falseRaw)
		if !result.Vulnerable || result.Severity != entity.SeverityCritical {
			t.Fatalf("a 50x content-length divergence must be flagged vulnerable")
		}
	})

	t.Run("identical responses are not vulnerable", func(t *testing.T) {
		result := &BooleanPairResult{}
		body := strings.Repeat("a", 200)
		trueRaw := rawResponse{status: 200, body: body}
		falseRaw := rawResponse{status: 200, body: body}
		classifyBooleanPair(result, trueRaw, falseRaw)
		if result.Vulnerable {
			t.Fatalf("identical true/false responses must not be flagged vulnerable")
		}
	})

	t.Run("status code divergence alone is vulnerable", func(t *testing.T) {
		result := &BooleanPairResult{}
		trueRaw := rawResponse{status: 200, body: "same"}
		falseRaw := rawResponse{status: 500, body: "same"}
		classifyBooleanPair(result, trueRaw, falseRaw)
		if !result.Vulnerable {
			t.Fatalf("a status code divergence must be flagged vulnerable even with identical bodies")
		}
	})
}

func TestParseFormBody(t *testing.T) {
	if _, ok := parseFormBody(""); ok {
		t.Fatal("empty body should report no params")
	}
	values, ok := parseFormBody("id=1")
	if !ok || values.Get("id") != "1" {
		t.Fatalf("got %v, ok=%v", values, ok)
	}
}

// TestRun_DetectsErrorBasedInjection uses a fake vulnerable endpoint: a `'`
// anywhere in the "id" param triggers a canned MySQL error message, exactly
// the signal-based behavior Run() is meant to catch without any timing
// dependency (unlike the time-based check, which needs a multi-second delay
// to exercise for real).
func TestRun_DetectsErrorBasedInjection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if strings.Contains(id, "'") {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "You have an error in your SQL syntax near '"+id+"'")
			return
		}
		fmt.Fprintf(w, "item #%s", id)
	}))
	defer srv.Close()

	report := Run(context.Background(), Request{
		Method: http.MethodGet,
		URL:    srv.URL + "?id=1",
	})

	if report.Summary.Critical == 0 {
		t.Fatalf("expected at least one critical (error-based) finding, got summary=%+v", report.Summary)
	}
	found := false
	for _, f := range report.Findings {
		if f.Vulnerable && f.Severity == entity.SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Fatal("expected at least one Finding with Vulnerable=true, Severity=Critical")
	}
}

// TestRun_DetectsBooleanBasedInjection fakes a classic boolean-blind SQLi:
// the app naively evaluates whatever condition follows "id=1 AND ..." and
// returns a big page for a true condition, an empty page for false — the
// divergence classifyBooleanPair is designed to catch.
func TestRun_DetectsBooleanBasedInjection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if strings.Contains(id, "1=1") {
			fmt.Fprint(w, strings.Repeat("item row ", 200))
			return
		}
		if strings.Contains(id, "1=2") {
			fmt.Fprint(w, "")
			return
		}
		fmt.Fprint(w, "item #1")
	}))
	defer srv.Close()

	report := Run(context.Background(), Request{
		Method: http.MethodGet,
		URL:    srv.URL + "?id=1",
	})

	vulnerablePair := false
	for _, p := range report.BooleanPairs {
		if p.Vulnerable {
			vulnerablePair = true
		}
	}
	if !vulnerablePair {
		t.Fatalf("expected at least one vulnerable boolean pair, got %+v", report.BooleanPairs)
	}
}

func TestRun_NoFalsePositiveAgainstOrdinaryEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "item #%s", r.URL.Query().Get("id"))
	}))
	defer srv.Close()

	report := Run(context.Background(), Request{
		Method: http.MethodGet,
		URL:    srv.URL + "?id=1",
	})

	if report.Summary.Vulnerable != 0 {
		t.Fatalf("an ordinary, non-vulnerable endpoint must not be flagged, got summary=%+v", report.Summary)
	}
}
