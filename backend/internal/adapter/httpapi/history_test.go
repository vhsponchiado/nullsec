package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"nullsec/backend/internal/entity"
)

func seedHistory(t *testing.T, h *harness, id, host, url string) {
	t.Helper()
	if err := h.history.Insert(&entity.Request{
		ID:        id,
		Method:    "GET",
		URL:       url,
		Host:      host,
		Headers:   "{}",
		Status:    200,
		Outcome:   "completed",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("seed history: %v", err)
	}
}

func TestHistory_ListEmpty(t *testing.T) {
	h := newHarness(t)

	rec := h.do(http.MethodGet, "/api/history", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Items []entity.RequestSummary `json:"items"`
		Total int                     `json:"total"`
		Page  int                     `json:"page"`
		Limit int                     `json:"limit"`
	}
	decode(t, rec, &body)
	if body.Total != 0 || len(body.Items) != 0 {
		t.Fatalf("expected an empty history, got %+v", body)
	}
	if body.Limit != 200 {
		t.Fatalf("default limit = %d, want 200", body.Limit)
	}
}

func TestHistory_ListAndGet(t *testing.T) {
	h := newHarness(t)
	seedHistory(t, h, "req-1", "example.com", "http://example.com/")

	listRec := h.do(http.MethodGet, "/api/history", nil)
	var list struct {
		Items []entity.RequestSummary `json:"items"`
		Total int                     `json:"total"`
	}
	decode(t, listRec, &list)
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("expected exactly one history item, got %+v", list)
	}

	getRec := h.do(http.MethodGet, "/api/history/req-1", nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get history item status = %d", getRec.Code)
	}
	var item entity.Request
	decode(t, getRec, &item)
	if item.ID != "req-1" || item.URL != "http://example.com/" {
		t.Fatalf("got item %+v", item)
	}
}

func TestHistory_GetUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/history/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an unknown id", rec.Code)
	}
}

func TestHistory_FilterByHost(t *testing.T) {
	h := newHarness(t)
	seedHistory(t, h, "req-a", "a.example", "http://a.example/")
	seedHistory(t, h, "req-b", "b.example", "http://b.example/")

	rec := h.do(http.MethodGet, "/api/history?host=a.example", nil)
	var list struct {
		Items []entity.RequestSummary `json:"items"`
		Total int                     `json:"total"`
	}
	decode(t, rec, &list)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].Host != "a.example" {
		t.Fatalf("host filter did not narrow correctly: %+v", list)
	}
}

func TestHistory_Pagination(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		seedHistory(t, h, string(rune('a'+i)), "example.com", "http://example.com/")
	}

	rec := h.do(http.MethodGet, "/api/history?limit=2&page=0", nil)
	var page0 struct {
		Items []entity.RequestSummary `json:"items"`
		Total int                     `json:"total"`
		Limit int                     `json:"limit"`
	}
	decode(t, rec, &page0)
	if page0.Total != 3 || len(page0.Items) != 2 || page0.Limit != 2 {
		t.Fatalf("page 0 (limit=2) = %+v", page0)
	}

	rec = h.do(http.MethodGet, "/api/history?limit=2&page=1", nil)
	var page1 struct {
		Items []entity.RequestSummary `json:"items"`
	}
	decode(t, rec, &page1)
	if len(page1.Items) != 1 {
		t.Fatalf("page 1 (limit=2) should have the remaining 1 item, got %+v", page1)
	}
}

func TestHistory_Clear(t *testing.T) {
	h := newHarness(t)
	seedHistory(t, h, "req-1", "example.com", "http://example.com/")

	rec := h.do(http.MethodDelete, "/api/history", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear status = %d", rec.Code)
	}

	listRec := h.do(http.MethodGet, "/api/history", nil)
	var list struct {
		Total int `json:"total"`
	}
	decode(t, listRec, &list)
	if list.Total != 0 {
		t.Fatalf("expected history to be empty after clear, total=%d", list.Total)
	}
}
