package httpapi_test

import (
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestPortscan_Presets(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/portscan/presets", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var presets struct {
		Top100  []map[string]any `json:"top100"`
		Top1000 []map[string]any `json:"top1000"`
	}
	decode(t, rec, &presets)
	if len(presets.Top100) == 0 || len(presets.Top1000) == 0 {
		t.Fatalf("expected non-empty port presets, got %+v", presets)
	}
}

func TestPortscan_StartAndComplete(t *testing.T) {
	h := newHarness(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	rec := h.doJSON(http.MethodPost, "/api/tools/portscan/start", map[string]any{
		"host":  host,
		"ports": []int{port},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", rec.Code, rec.Body.String())
	}
	var start struct {
		ScanID string `json:"scan_id"`
		Total  int    `json:"total"`
	}
	decode(t, rec, &start)
	if start.ScanID == "" || start.Total != 1 {
		t.Fatalf("start response = %+v", start)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		statusRec := h.do(http.MethodGet, "/api/tools/portscan/"+start.ScanID, nil)
		var status struct {
			Status  string `json:"status"`
			Results []struct {
				Port  int    `json:"port"`
				State string `json:"state"`
			} `json:"results"`
		}
		decode(t, statusRec, &status)
		if status.Status == "done" {
			if len(status.Results) != 1 || status.Results[0].Port != port || status.Results[0].State != "open" {
				t.Fatalf("results = %+v, want a single open result for port %d", status.Results, port)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan never reached status=done")
}

func TestPortscan_StopUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/tools/portscan/does-not-exist/stop", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestPortscan_StatusUnknownID(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/tools/portscan/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
