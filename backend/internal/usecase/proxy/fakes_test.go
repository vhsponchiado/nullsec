package proxy

import (
	"net/http"
	"sync"

	"nullsec/backend/internal/entity"
)

// fakeHistory is an in-memory entity.HistoryRepository — enough to assert
// what Forward actually persisted without a real SQLite file.
type fakeHistory struct {
	mu    sync.Mutex
	items []entity.Request
}

func (f *fakeHistory) Insert(r *entity.Request) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = append(f.items, *r)
	return nil
}

func (f *fakeHistory) Get(id string) (*entity.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.items {
		if r.ID == id {
			cp := r
			return &cp, nil
		}
	}
	return nil, errNotFound
}

func (f *fakeHistory) List(entity.RequestFilter) ([]entity.Request, error) { return nil, nil }
func (f *fakeHistory) ListSummary(entity.RequestFilter) ([]entity.RequestSummary, error) {
	return nil, nil
}
func (f *fakeHistory) Count(entity.RequestFilter) (int, error) { return 0, nil }
func (f *fakeHistory) Clear() error                            { return nil }

func (f *fakeHistory) all() []entity.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]entity.Request, len(f.items))
	copy(out, f.items)
	return out
}

type notFoundError struct{}

func (notFoundError) Error() string { return "not found" }

var errNotFound = notFoundError{}

// broadcastCall records one Broadcast invocation for assertions.
type broadcastCall struct {
	eventType string
	payload   any
}

// fakeBroadcaster is an in-memory entity.Broadcaster. onBroadcast, when set,
// runs synchronously inside Broadcast — tests use it to react to
// EventRequestIntercepted/EventResponseIntercepted (e.g. resolving the
// pending entry) without racing against Forward's own goroutine.
type fakeBroadcaster struct {
	mu          sync.Mutex
	calls       []broadcastCall
	onBroadcast func(eventType string, payload any)
}

func (f *fakeBroadcaster) Broadcast(eventType string, payload any) {
	f.mu.Lock()
	f.calls = append(f.calls, broadcastCall{eventType, payload})
	hook := f.onBroadcast
	f.mu.Unlock()

	if hook != nil {
		hook(eventType, payload)
	}
}

func (f *fakeBroadcaster) events() []broadcastCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]broadcastCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// roundTripFunc adapts a plain function to http.RoundTripper, so each test
// can stub the outbound round trip without a real network call.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
