package proxy

import (
	"net/http"
	"sync"
	"time"
)

// interceptTimeout is a var, not a const, solely so tests can shrink it —
// production behavior is unaffected, it's always 5 minutes outside tests.
var interceptTimeout = 5 * time.Minute

type PendingRequest struct {
	ID        string      `json:"id"`
	Method    string      `json:"method"`
	URL       string      `json:"url"`
	Headers   http.Header `json:"headers"`
	Body      string      `json:"body"`
	CreatedAt time.Time   `json:"created_at"`
}

type Decision struct {
	Action  string      `json:"action"` // "forward" or "drop"
	Method  string      `json:"method"`
	URL     string      `json:"url"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
}

type pendingEntry struct {
	req PendingRequest
	ch  chan Decision
}

type Interceptor struct {
	mu      sync.Mutex
	enabled bool
	pending map[string]*pendingEntry
}

func NewInterceptor() *Interceptor {
	return &Interceptor{pending: make(map[string]*pendingEntry)}
}

func (i *Interceptor) SetEnabled(v bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.enabled = v
}

func (i *Interceptor) Enabled() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.enabled
}

func (i *Interceptor) Register(req PendingRequest) chan Decision {
	ch := make(chan Decision, 1)
	i.mu.Lock()
	i.pending[req.ID] = &pendingEntry{req: req, ch: ch}
	i.mu.Unlock()
	return ch
}

func (i *Interceptor) Cleanup(id string) {
	i.mu.Lock()
	delete(i.pending, id)
	i.mu.Unlock()
}

func (i *Interceptor) Get(id string) (PendingRequest, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	e, ok := i.pending[id]
	if !ok {
		return PendingRequest{}, false
	}
	return e.req, true
}

func (i *Interceptor) List() []PendingRequest {
	i.mu.Lock()
	defer i.mu.Unlock()
	out := make([]PendingRequest, 0, len(i.pending))
	for _, e := range i.pending {
		out = append(out, e.req)
	}
	return out
}

// Resolve delivers a decision to the pending request's handler goroutine.
// Returns false if no such pending request exists (already resolved/timed out).
func (i *Interceptor) Resolve(id string, d Decision) bool {
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
