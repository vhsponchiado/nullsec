package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestServer(h *Hub) (*httptest.Server, string) {
	srv := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, wsURL
}

func dial(t *testing.T, wsURL string) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial %s: %v", wsURL, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestHub_BroadcastDeliversToConnectedClient(t *testing.T) {
	h := NewHub()
	srv, wsURL := newTestServer(h)
	defer srv.Close()

	conn := dial(t, wsURL)
	waitForClientCount(t, h, 1)

	h.Broadcast("request_completed", map[string]string{"id": "abc"})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var got Event
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
	if got.Type != "request_completed" {
		t.Fatalf("Type = %q, want request_completed", got.Type)
	}
	payload, ok := got.Payload.(map[string]any)
	if !ok || payload["id"] != "abc" {
		t.Fatalf("Payload = %+v, want {id: abc}", got.Payload)
	}
}

func TestHub_BroadcastReachesEveryConnectedClient(t *testing.T) {
	h := NewHub()
	srv, wsURL := newTestServer(h)
	defer srv.Close()

	const n = 3
	conns := make([]*websocket.Conn, n)
	for i := range conns {
		conns[i] = dial(t, wsURL)
	}
	waitForClientCount(t, h, n)

	h.Broadcast("ping", nil)

	for i, conn := range conns {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Fatalf("client %d never received the broadcast: %v", i, err)
		}
	}
}

func TestHub_DisconnectedClientIsRemoved(t *testing.T) {
	h := NewHub()
	srv, wsURL := newTestServer(h)
	defer srv.Close()

	conn := dial(t, wsURL)
	waitForClientCount(t, h, 1)

	conn.Close()
	waitForClientCount(t, h, 0)
}

func TestHub_BroadcastWithNoClientsDoesNotPanic(t *testing.T) {
	h := NewHub()
	h.Broadcast("nobody_listening", map[string]string{"x": "y"})
}

func TestHub_BroadcastToDisconnectedClientPrunesIt(t *testing.T) {
	h := NewHub()
	srv, wsURL := newTestServer(h)
	defer srv.Close()

	live := dial(t, wsURL)

	// A second client that goes away without a clean close handshake —
	// this exercises the write-error cleanup path inside Broadcast itself,
	// not just the read-loop cleanup in ServeHTTP.
	dead, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	waitForClientCount(t, h, 2)
	dead.UnderlyingConn().Close()

	deadline := time.Now().Add(2 * time.Second)
	for {
		h.Broadcast("probe", nil)
		_ = live.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		if _, _, err := live.ReadMessage(); err != nil {
			t.Fatalf("live client should keep receiving broadcasts: %v", err)
		}
		if clientCount(h) == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("dead client was never pruned from the hub")
		}
	}
}

func clientCount(h *Hub) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func waitForClientCount(t *testing.T, h *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if clientCount(h) == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("client count = %d, want %d", clientCount(h), want)
}
