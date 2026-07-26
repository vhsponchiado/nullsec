package wpcheck

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"nullsec/backend/internal/entity"
)

func TestClassify_WPLoginFound(t *testing.T) {
	res := &Result{Status: 200}
	classify(res, check{Path: "/wp-login.php"}, `<form id="loginform"><input name="user_login"></form>`, "")
	if !res.Found || res.Severity != entity.SeverityWarning {
		t.Fatalf("res = %+v, want Found=true Severity=warning", res)
	}
}

func TestClassify_WPLoginNotFound(t *testing.T) {
	res := &Result{Status: 404}
	classify(res, check{Path: "/wp-login.php"}, "not found", "")
	if res.Found {
		t.Fatalf("res = %+v, want Found=false for a 404", res)
	}
}

func TestClassify_XMLRPC(t *testing.T) {
	res := &Result{Status: 405}
	classify(res, check{Path: "/xmlrpc.php"}, "", "")
	if !res.Found {
		t.Fatal("a 405 on xmlrpc.php should be classified as found")
	}
}

func TestClassify_UserEnumerationCritical(t *testing.T) {
	res := &Result{Status: 200}
	classify(res, check{Path: "/wp-json/wp/v2/users"}, `[{"id":1,"slug":"admin"}]`, "")
	if !res.Found || res.Severity != entity.SeverityCritical {
		t.Fatalf("res = %+v, want Found=true Severity=critical", res)
	}
}

func TestClassify_AuthorRedirectRevealsUsername(t *testing.T) {
	res := &Result{Status: 301}
	classify(res, check{Path: "/?author=1"}, "", "https://example.com/author/admin/")
	if !res.Found || res.Severity != entity.SeverityCritical {
		t.Fatalf("res = %+v, want Found=true Severity=critical", res)
	}
	if res.Note == "" {
		t.Fatal("expected a note including the revealed username redirect")
	}
}

func TestClassify_AuthorRedirectElsewhereNotFound(t *testing.T) {
	res := &Result{Status: 302}
	classify(res, check{Path: "/?author=1"}, "", "https://example.com/")
	if res.Found {
		t.Fatal("a redirect that isn't to /author/ should not be classified as found")
	}
}

func TestClassify_SensitiveBackupFiles(t *testing.T) {
	for _, path := range []string{"/wp-config.php.bak", "/wp-config.php.save", "/wp-config.sql", "/wp-config.zip"} {
		t.Run(path, func(t *testing.T) {
			res := &Result{Status: 200}
			classify(res, check{Path: path}, "", "")
			if !res.Found || res.Severity != entity.SeverityCritical {
				t.Fatalf("res = %+v, want Found=true Severity=critical for %s", res, path)
			}
		})
	}
}

func TestClassify_DebugLogExposed(t *testing.T) {
	res := &Result{Status: 200}
	classify(res, check{Path: "/wp-content/debug.log"}, "", "")
	if !res.Found || res.Severity != entity.SeverityCritical {
		t.Fatalf("res = %+v, want Found=true Severity=critical", res)
	}
}

func TestClassify_ReadmeVersionExtracted(t *testing.T) {
	res := &Result{Status: 200}
	classify(res, check{Path: "/readme.html"}, "WordPress Version 6.4.2", "")
	if !res.Found || res.Note != "Version exposed: 6.4.2" {
		t.Fatalf("res = %+v, want the version number extracted into Note", res)
	}
}

func TestClassify_DirectoryListingOpenVsBlocked(t *testing.T) {
	open := &Result{Status: 200}
	classify(open, check{Path: "/wp-content/uploads/"}, "", "")
	if !open.Found || open.Note != "directory listing open" {
		t.Fatalf("res = %+v, want directory listing open", open)
	}

	blocked := &Result{Status: 403}
	classify(blocked, check{Path: "/wp-content/uploads/"}, "", "")
	if !blocked.Found || blocked.Note != "directory exists (listing blocked)" {
		t.Fatalf("res = %+v, want directory exists but blocked", blocked)
	}

	missing := &Result{Status: 404}
	classify(missing, check{Path: "/wp-content/uploads/"}, "", "")
	if missing.Found {
		t.Fatalf("res = %+v, want not found for a 404", missing)
	}
}

func TestClassify_DefaultFallsBackToStatus200(t *testing.T) {
	res := &Result{Status: 200}
	classify(res, check{Path: "/some/unlisted/path"}, "", "")
	if !res.Found {
		t.Fatal("the default case should treat any 200 as found")
	}
}

// TestRun_EndToEnd exercises the real concurrent Run() against a local
// server that only recognizes two of the checked paths, confirming the
// summary counts line up with what the per-path classification produced.
func TestRun_EndToEnd(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wp-login.php":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<input name="user_login">`))
		case "/wp-config.php.bak":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("DB_PASSWORD=hunter2"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer target.Close()

	results, summary := Run(t.Context(), target.URL)

	if summary.Checked != len(checks) {
		t.Fatalf("Checked = %d, want %d (every declared check probed)", summary.Checked, len(checks))
	}
	if summary.Found != 2 {
		t.Fatalf("Found = %d, want 2 (wp-login.php + the backup file)", summary.Found)
	}
	if summary.Critical != 1 {
		t.Fatalf("Critical = %d, want 1 (the exposed backup)", summary.Critical)
	}
	if len(results) != len(checks) {
		t.Fatalf("len(results) = %d, want %d", len(results), len(checks))
	}
}
