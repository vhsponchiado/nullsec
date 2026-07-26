package proxy

import (
	"reflect"
	"testing"
)

func TestInterceptor_EnabledToggle(t *testing.T) {
	i := NewInterceptor()
	if i.Enabled() {
		t.Fatal("a fresh Interceptor should start disabled")
	}
	i.SetEnabled(true)
	if !i.Enabled() {
		t.Fatal("SetEnabled(true) should make Enabled() report true")
	}
	i.SetEnabled(false)
	if i.Enabled() {
		t.Fatal("SetEnabled(false) should make Enabled() report false")
	}
}

func TestInterceptor_RegisterGetList(t *testing.T) {
	i := NewInterceptor()
	req := PendingRequest{ID: "req-1", Method: "GET", URL: "http://example.com"}

	if _, ok := i.Get("req-1"); ok {
		t.Fatal("Get should report false before Register")
	}

	i.Register(req)

	got, ok := i.Get("req-1")
	if !ok || got.ID != "req-1" || got.URL != "http://example.com" {
		t.Fatalf("Get after Register = %+v, %v", got, ok)
	}

	list := i.List()
	if len(list) != 1 || list[0].ID != "req-1" {
		t.Fatalf("List = %+v, want a single entry for req-1", list)
	}
}

func TestInterceptor_ResolveDeliversDecisionAndRemoves(t *testing.T) {
	i := NewInterceptor()
	ch := i.Register(PendingRequest{ID: "req-1"})

	decision := Decision{Action: "forward", Method: "POST", URL: "http://edited.example"}
	if !i.Resolve("req-1", decision) {
		t.Fatal("Resolve on a pending id should return true")
	}

	select {
	case got := <-ch:
		if !reflect.DeepEqual(got, decision) {
			t.Fatalf("channel delivered %+v, want %+v", got, decision)
		}
	default:
		t.Fatal("Resolve should deliver the decision on the registered channel synchronously")
	}

	if _, ok := i.Get("req-1"); ok {
		t.Fatal("Resolve should remove the entry from pending")
	}
}

func TestInterceptor_ResolveUnknownIDReturnsFalse(t *testing.T) {
	i := NewInterceptor()
	if i.Resolve("does-not-exist", Decision{Action: "drop"}) {
		t.Fatal("Resolve on an unregistered id should return false")
	}
}

func TestInterceptor_ResolveTwiceReturnsFalseSecondTime(t *testing.T) {
	i := NewInterceptor()
	i.Register(PendingRequest{ID: "req-1"})

	if !i.Resolve("req-1", Decision{Action: "drop"}) {
		t.Fatal("first Resolve should succeed")
	}
	if i.Resolve("req-1", Decision{Action: "drop"}) {
		t.Fatal("second Resolve on an already-resolved id should return false, not double-deliver")
	}
}

func TestInterceptor_Cleanup(t *testing.T) {
	i := NewInterceptor()
	i.Register(PendingRequest{ID: "req-1"})
	i.Cleanup("req-1")

	if _, ok := i.Get("req-1"); ok {
		t.Fatal("Cleanup should remove the pending entry")
	}
	// Cleaning up an id that was never registered (or already cleaned up)
	// must not panic — this is exactly what Forward's timeout path does.
	i.Cleanup("req-1")
}
