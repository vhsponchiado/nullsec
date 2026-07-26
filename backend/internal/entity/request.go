package entity

import "time"

// Request is a single completed (or dropped) proxy transaction — the core
// record behind Proxy History, and the raw material the passive scanner
// reads back.
type Request struct {
	ID          string    `json:"id"`
	Method      string    `json:"method"`
	URL         string    `json:"url"`
	Host        string    `json:"host"`
	Headers     string    `json:"headers"`
	Body        string    `json:"body"`
	Status      int       `json:"status"`
	RespHeaders string    `json:"resp_headers"`
	RespBody    string    `json:"resp_body"`
	Size        int       `json:"size"`
	DurationMs  int64     `json:"duration_ms"`
	Outcome     string    `json:"outcome"` // "completed" or "dropped"
	CreatedAt   time.Time `json:"created_at"`
}

// RequestSummary is the lightweight projection of a Request used for list
// views — it deliberately omits headers/body/resp_headers/resp_body, which
// can be arbitrarily large (a captured image, a JS bundle) and aren't
// needed to render a row. Fetch the full Request only once a row is
// actually selected.
type RequestSummary struct {
	ID         string    `json:"id"`
	Method     string    `json:"method"`
	URL        string    `json:"url"`
	Host       string    `json:"host"`
	Status     int       `json:"status"`
	Size       int       `json:"size"`
	DurationMs int64     `json:"duration_ms"`
	Outcome    string    `json:"outcome"`
	CreatedAt  time.Time `json:"created_at"`
}

// Summary strips the (potentially large) headers/body/resp_headers/resp_body
// fields — this is what gets broadcast over the WebSocket on every proxied
// request, so every connected client isn't forced to receive and JSON-parse
// a full captured body (an image, a JS bundle, ...) for traffic it may not
// even be looking at. The frontend fetches the full Request on demand only
// once a row is actually selected.
func (r *Request) Summary() RequestSummary {
	return RequestSummary{
		ID:         r.ID,
		Method:     r.Method,
		URL:        r.URL,
		Host:       r.Host,
		Status:     r.Status,
		Size:       r.Size,
		DurationMs: r.DurationMs,
		Outcome:    r.Outcome,
		CreatedAt:  r.CreatedAt,
	}
}

// RequestFilter narrows a history query — used by both ListSummary (the
// history table) and Count (its pagination total).
type RequestFilter struct {
	Host   string
	Method string
	Status int
	Limit  int
	Offset int
}

// HistoryRepository is the persistence port for proxy history. It's
// consumed by more than one use case — proxy.Forward records every
// completed/dropped request, spider.Crawl records every page it fetches,
// and the frontend's Proxy History view reads it back — so the interface
// lives here in entity rather than inside a single usecase package.
//
// The only implementation today is adapter/sqliterepo, but nothing above
// this line knows that.
type HistoryRepository interface {
	Insert(r *Request) error
	Get(id string) (*Request, error)
	List(f RequestFilter) ([]Request, error)
	ListSummary(f RequestFilter) ([]RequestSummary, error)
	Count(f RequestFilter) (int, error)
	Clear() error
}
