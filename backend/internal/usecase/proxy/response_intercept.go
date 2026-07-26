package proxy

import (
	"net/http"
	"sync"
)

// PendingResponse is a response held for review before being relayed to the
// client, along with enough of the originating request to identify it in the UI.
type PendingResponse struct {
	ID            string      `json:"id"`
	RequestMethod string      `json:"request_method"`
	RequestURL    string      `json:"request_url"`
	Status        int         `json:"status"`
	Headers       http.Header `json:"headers"`
	Body          string      `json:"body"`
}

type ResponseDecision struct {
	Action  string      `json:"action"` // "forward" or "drop"
	Status  int         `json:"status"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
}

type pendingResponseEntry struct {
	resp PendingResponse
	ch   chan ResponseDecision
}

// ResponseInterceptor mirrors Interceptor but pauses responses on their way
// back to the client instead of requests on their way out — Burp calls this
// "Intercept server responses".
type ResponseInterceptor struct {
	mu      sync.Mutex
	enabled bool
	pending map[string]*pendingResponseEntry
}

func NewResponseInterceptor() *ResponseInterceptor {
	return &ResponseInterceptor{pending: make(map[string]*pendingResponseEntry)}
}

func (i *ResponseInterceptor) SetEnabled(v bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.enabled = v
}

func (i *ResponseInterceptor) Enabled() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.enabled
}

func (i *ResponseInterceptor) Register(resp PendingResponse) chan ResponseDecision {
	ch := make(chan ResponseDecision, 1)
	i.mu.Lock()
	i.pending[resp.ID] = &pendingResponseEntry{resp: resp, ch: ch}
	i.mu.Unlock()
	return ch
}

func (i *ResponseInterceptor) Cleanup(id string) {
	i.mu.Lock()
	delete(i.pending, id)
	i.mu.Unlock()
}

func (i *ResponseInterceptor) Get(id string) (PendingResponse, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	e, ok := i.pending[id]
	if !ok {
		return PendingResponse{}, false
	}
	return e.resp, true
}

func (i *ResponseInterceptor) List() []PendingResponse {
	i.mu.Lock()
	defer i.mu.Unlock()
	out := make([]PendingResponse, 0, len(i.pending))
	for _, e := range i.pending {
		out = append(out, e.resp)
	}
	return out
}

func (i *ResponseInterceptor) Resolve(id string, d ResponseDecision) bool {
	i.mu.Lock()
	e, ok := i.pending[id]
	if ok {
		delete(i.pending, id)
	}
	i.mu.Unlock()
	if !ok {
		return false
	}
	e.ch <- d
	return true
}
