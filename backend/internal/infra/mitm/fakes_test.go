package mitm

import (
	"net/http"
	"sync"

	"nullsec/backend/internal/entity"
)

// fakeHistory is a minimal in-memory entity.HistoryRepository — only Insert
// is exercised by proxy.Forwarder.
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
func (f *fakeHistory) Get(string) (*entity.Request, error)                 { return nil, nil }
func (f *fakeHistory) List(entity.RequestFilter) ([]entity.Request, error) { return nil, nil }
func (f *fakeHistory) ListSummary(entity.RequestFilter) ([]entity.RequestSummary, error) {
	return nil, nil
}
func (f *fakeHistory) Count(entity.RequestFilter) (int, error) { return 0, nil }
func (f *fakeHistory) Clear() error                            { return nil }

// fakeBroadcaster is an entity.Broadcaster. onBroadcast, when set, runs
// synchronously inside Broadcast — tests use it to react to
// EventRequestIntercepted (e.g. resolving the pending entry) without racing
// against Forward's own goroutine.
type fakeBroadcaster struct {
	onBroadcast func(eventType string, payload any)
}

func (f fakeBroadcaster) Broadcast(eventType string, payload any) {
	if f.onBroadcast != nil {
		f.onBroadcast(eventType, payload)
	}
}

// roundTripFunc adapts a plain function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
