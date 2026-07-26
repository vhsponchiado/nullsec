package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIntruder_StartAndComplete(t *testing.T) {
	h := newHarness(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.Query().Get("id")))
	}))
	defer target.Close()

	rec := h.doJSON(http.MethodPost, "/api/tools/intruder/start", map[string]any{
		"template": map[string]any{
			"method": "GET",
			"url":    target.URL + "/?id=§x§",
		},
		"attack_type":  "sniper",
		"payload_sets": [][]string{{"a", "b"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", rec.Code, rec.Body.String())
	}
	var start struct {
		AttackID string `json:"attack_id"`
		Total    int    `json:"total"`
	}
	decode(t, rec, &start)
	if start.AttackID == "" || start.Total != 2 {
		t.Fatalf("start response = %+v, want total=2 (1 position x 2 payloads)", start)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		statusRec := h.do(http.MethodGet, "/api/tools/intruder/"+start.AttackID, nil)
		var status struct {
			Status  string `json:"status"`
			Results []struct {
				Index int `json:"index"`
			} `json:"results"`
		}
		decode(t, statusRec, &status)
		if status.Status == "done" {
			if len(status.Results) != 2 {
				t.Fatalf("expected 2 results once done, got %+v", status.Results)
			}
			resultRec := h.do(http.MethodGet, "/api/tools/intruder/"+start.AttackID+"/results/0", nil)
			if resultRec.Code != http.StatusOK {
				t.Fatalf("result status = %d body=%s", resultRec.Code, resultRec.Body.String())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("attack never reached status=done")
}

func TestIntruder_StartNoPositionsMarked(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/intruder/start", map[string]any{
		"template":     map[string]any{"method": "GET", "url": "http://example.com/"},
		"attack_type":  "sniper",
		"payload_sets": [][]string{{"a"}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when no §…§ position is marked", rec.Code)
	}
}

func TestIntruder_StopUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/tools/intruder/does-not-exist/stop", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestIntruder_StatusUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/intruder/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestIntruder_ResultInvalidIndex(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/intruder/some-id/results/not-a-number", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a non-numeric index", rec.Code)
	}
}

func TestIntruder_Wordlists(t *testing.T) {
	h := newHarness(t)

	rec := h.do(http.MethodGet, "/api/tools/wordlist/passwords", nil)
	var passwords []string
	decode(t, rec, &passwords)
	if len(passwords) == 0 {
		t.Fatal("expected a non-empty built-in password wordlist")
	}

	rec = h.do(http.MethodGet, "/api/tools/wordlist/usernames", nil)
	var usernames []string
	decode(t, rec, &usernames)
	if len(usernames) == 0 {
		t.Fatal("expected a non-empty built-in username wordlist")
	}

	rec = h.do(http.MethodGet, "/api/tools/wordlist/credentials", nil)
	var creds struct {
		Usernames []string `json:"usernames"`
		Passwords []string `json:"passwords"`
	}
	decode(t, rec, &creds)
	if len(creds.Usernames) == 0 || len(creds.Usernames) != len(creds.Passwords) {
		t.Fatalf("credential lists = %+v, want equal non-empty paired lists", creds)
	}
}
