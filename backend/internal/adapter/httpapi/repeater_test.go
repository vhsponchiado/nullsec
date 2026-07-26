package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	proxyuc "nullsec/backend/internal/usecase/proxy"
)

func TestRepeater_SendDirect_Success(t *testing.T) {
	h := newHarness(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "1")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello from target"))
	}))
	defer target.Close()

	rec := h.doJSON(http.MethodPost, "/api/repeater/send", map[string]any{
		"method": "GET",
		"url":    target.URL,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("repeater send status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
		Error  string `json:"error"`
	}
	decode(t, rec, &resp)
	if resp.Error != "" {
		t.Fatalf("unexpected Error = %q", resp.Error)
	}
	if resp.Status != http.StatusCreated || resp.Body != "hello from target" {
		t.Fatalf("resp = %+v", resp)
	}

	// A direct send (the default — no interceptResponse) never touches the
	// proxy pipeline, so it must not create a history entry.
	listRec := h.do(http.MethodGet, "/api/history", nil)
	var list struct {
		Total int `json:"total"`
	}
	decode(t, listRec, &list)
	if list.Total != 0 {
		t.Fatalf("history.Total = %d, want 0 for a direct repeater send", list.Total)
	}
}

func TestRepeater_SendDirect_TransportErrorStillReturns200(t *testing.T) {
	h := newHarness(t)
	closedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := closedServer.URL
	closedServer.Close() // now guaranteed refused: nothing listens on this port anymore

	rec := h.doJSON(http.MethodPost, "/api/repeater/send", map[string]any{
		"method": "GET",
		"url":    closedURL,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 even when the send itself fails", rec.Code)
	}
	var resp struct {
		Error string `json:"error"`
	}
	decode(t, rec, &resp)
	if resp.Error == "" {
		t.Fatal("expected a non-empty Error field for a connection that was refused")
	}
}

func TestRepeater_SendMissingMethodOrURL(t *testing.T) {
	h := newHarness(t)

	rec := h.doJSON(http.MethodPost, "/api/repeater/send", map[string]any{"method": "GET"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when url is missing", rec.Code)
	}

	rec = h.doJSON(http.MethodPost, "/api/repeater/send", map[string]any{"url": "http://example.com/"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when method is missing", rec.Code)
	}
}

// TestRepeater_SendViaProxy_ForcesResponseIntercept exercises the
// "interceptResponse" flag — it routes through the real proxy pipeline
// (Forwarder, using the harness's fake transport) and forces the response
// into the Intercept > Responses queue rather than returning it directly,
// which is what lets a single Repeater send be reviewed/edited without
// flipping the global response-intercept toggle for all live traffic.
func TestRepeater_SendViaProxy_ForcesResponseIntercept(t *testing.T) {
	h := newHarness(t)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- h.doJSON(http.MethodPost, "/api/repeater/send", map[string]any{
			"method":            "GET",
			"url":               "http://example.com/",
			"interceptResponse": true,
		})
	}()

	var id string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if list := h.respIntercept.List(); len(list) == 1 {
			id = list[0].ID
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if id == "" {
		t.Fatal("timed out waiting for the response to be held for intercept")
	}

	if !h.respIntercept.Resolve(id, proxyuc.ResponseDecision{Action: "forward", Status: 200, Body: "held response"}) {
		t.Fatal("Resolve on the pending response failed")
	}

	select {
	case rec := <-done:
		if rec.Code != http.StatusOK {
			t.Fatalf("repeater send status = %d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Status int    `json:"status"`
			Body   string `json:"body"`
		}
		decode(t, rec, &resp)
		if resp.Status != 200 || resp.Body != "held response" {
			t.Fatalf("resp = %+v, want the edited/forwarded values", resp)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("repeater send never returned after the held response was resolved")
	}
}
