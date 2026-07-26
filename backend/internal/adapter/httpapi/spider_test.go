package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSpider_StartAndComplete(t *testing.T) {
	h := newHarness(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Home</title></head><body>no links here</body></html>`))
	}))
	defer target.Close()

	rec := h.doJSON(http.MethodPost, "/api/tools/spider/start", map[string]any{
		"url":       target.URL,
		"max_pages": 1,
		"max_depth": 1,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", rec.Code, rec.Body.String())
	}
	var start struct {
		SpiderID string `json:"spider_id"`
	}
	decode(t, rec, &start)
	if start.SpiderID == "" {
		t.Fatal("expected a non-empty spider_id")
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		statusRec := h.do(http.MethodGet, "/api/tools/spider/"+start.SpiderID, nil)
		var status struct {
			Status string `json:"status"`
			Pages  []struct {
				URL   string `json:"url"`
				Title string `json:"title"`
			} `json:"pages"`
		}
		decode(t, statusRec, &status)
		if status.Status == "done" {
			if len(status.Pages) != 1 || status.Pages[0].Title != "Home" {
				t.Fatalf("pages = %+v, want a single crawled page titled Home", status.Pages)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("spider never reached status=done")
}

func TestSpider_StartInvalidURL(t *testing.T) {
	h := newHarness(t)
	rec := h.doJSON(http.MethodPost, "/api/tools/spider/start", map[string]any{"url": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an empty URL", rec.Code)
	}
}

func TestSpider_StopUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/tools/spider/does-not-exist/stop", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestSpider_StatusUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/spider/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
