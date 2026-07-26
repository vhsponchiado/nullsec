package proxy

import (
	"reflect"
	"testing"
)

func TestResponseInterceptor_EnabledToggle(t *testing.T) {
	i := NewResponseInterceptor()
	if i.Enabled() {
		t.Fatal("a fresh ResponseInterceptor should start disabled")
	}
	i.SetEnabled(true)
	if !i.Enabled() {
		t.Fatal("SetEnabled(true) should make Enabled() report true")
	}
}

func TestResponseInterceptor_RegisterGetList(t *testing.T) {
	i := NewResponseInterceptor()
	resp := PendingResponse{ID: "resp-1", Status: 200, RequestURL: "http://example.com"}

	if _, ok := i.Get("resp-1"); ok {
		t.Fatal("Get should report false before Register")
	}

	i.Register(resp)

	got, ok := i.Get("resp-1")
	if !ok || got.ID != "resp-1" || got.Status != 200 {
		t.Fatalf("Get after Register = %+v, %v", got, ok)
	}

	list := i.List()
	if len(list) != 1 || list[0].ID != "resp-1" {
		t.Fatalf("List = %+v, want a single entry for resp-1", list)
	}
}

func TestResponseInterceptor_ResolveDeliversDecisionAndRemoves(t *testing.T) {
	i := NewResponseInterceptor()
	ch := i.Register(PendingResponse{ID: "resp-1"})

	decision := ResponseDecision{Action: "forward", Status: 201, Body: "edited"}
	if !i.Resolve("resp-1", decision) {
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

	if _, ok := i.Get("resp-1"); ok {
		t.Fatal("Resolve should remove the entry from pending")
	}
}

func TestResponseInterceptor_ResolveUnknownIDReturnsFalse(t *testing.T) {
	i := NewResponseInterceptor()
	if i.Resolve("does-not-exist", ResponseDecision{Action: "drop"}) {
		t.Fatal("Resolve on an unregistered id should return false")
	}
}

func TestResponseInterceptor_Cleanup(t *testing.T) {
	i := NewResponseInterceptor()
	i.Register(PendingResponse{ID: "resp-1"})
	i.Cleanup("resp-1")

	if _, ok := i.Get("resp-1"); ok {
		t.Fatal("Cleanup should remove the pending entry")
	}
}
