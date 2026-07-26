package portscan

import (
	"net"
	"testing"
	"time"

	"nullsec/backend/internal/adapter/ws"
	"nullsec/backend/internal/entity"
	"nullsec/backend/internal/infra/upstream"
)

func TestSanitizeBanner(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "SSH-2.0-OpenSSH_9.6", "SSH-2.0-OpenSSH_9.6"},
		{"trims surrounding whitespace", "  hello  \n", "hello"},
		{"strips ANSI escape (terminal injection)", "hello\x1b[31mRED\x1b[0mworld", "hello[31mRED[0mworld"},
		{"strips NUL and other control bytes", "a\x00b\x01c\x7fd", "abcd"},
		{"keeps internal newlines and tabs", "line1\nline2\tcol2", "line1\nline2\tcol2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeBanner(tc.in); got != tc.want {
				t.Errorf("sanitizeBanner(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestManagerStart_Validation(t *testing.T) {
	m := NewManager(ws.NewHub(), upstream.NewManager())

	if _, _, err := m.Start(StartRequest{Ports: []int{80}}); err == nil {
		t.Error("expected an error for a missing host")
	}
	if _, _, err := m.Start(StartRequest{Host: "example.com"}); err == nil {
		t.Error("expected an error for an empty port list")
	}
	if _, _, err := m.Start(StartRequest{Host: "example.com", Ports: []int{0}}); err == nil {
		t.Error("expected an error for a port below MinPort")
	}
	if _, _, err := m.Start(StartRequest{Host: "example.com", Ports: []int{70000}}); err == nil {
		t.Error("expected an error for a port above MaxPort")
	}

	id, total, err := m.Start(StartRequest{Host: "127.0.0.1", Ports: []int{1}})
	if err != nil {
		t.Fatalf("unexpected error on a valid request: %v", err)
	}
	if id == "" || total != 1 {
		t.Fatalf("got id=%q total=%d, want a non-empty id and total=1", id, total)
	}
	m.Stop(id) // don't leave the scan's goroutine dialing after the test returns
}

// TestManagerScan_ClassifiesOpenAndClosedPorts runs a real (localhost-only)
// scan against one port we know is open (a listener this test controls) and
// one we know is closed (a listener opened and immediately closed, so the
// OS refuses new connections to it) — the two states the frontend actually
// renders differently.
func TestManagerScan_ClassifiesOpenAndClosedPorts(t *testing.T) {
	openLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open a test listener: %v", err)
	}
	defer openLn.Close()
	openPort := openLn.Addr().(*net.TCPAddr).Port

	closedLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open a throwaway listener: %v", err)
	}
	closedPort := closedLn.Addr().(*net.TCPAddr).Port
	closedLn.Close() // now refused: nothing is listening on this port anymore

	m := NewManager(ws.NewHub(), upstream.NewManager())
	id, _, err := m.Start(StartRequest{
		Host:      "127.0.0.1",
		Ports:     []int{openPort, closedPort},
		TimeoutMs: 500,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var results []Result
	for time.Now().Before(deadline) {
		status, _, r, ok := m.Snapshot(id)
		if !ok {
			t.Fatal("Snapshot: scan id vanished")
		}
		if status == entity.ScanDone {
			results = r
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results once the scan finished, got %d: %+v", len(results), results)
	}

	var gotOpen, gotClosed bool
	for _, r := range results {
		switch r.Port {
		case openPort:
			gotOpen = r.State == StateOpen
		case closedPort:
			gotClosed = r.State == StateClosed
		}
	}
	if !gotOpen {
		t.Errorf("port %d (a real open listener) was not classified StateOpen: %+v", openPort, results)
	}
	if !gotClosed {
		t.Errorf("port %d (a closed listener) was not classified StateClosed: %+v", closedPort, results)
	}
}
