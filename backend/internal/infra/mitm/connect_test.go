package mitm

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nullsec/backend/internal/infra/ca"
	proxyuc "nullsec/backend/internal/usecase/proxy"
)

func TestPort(t *testing.T) {
	cases := []struct {
		hostport string
		want     string
	}{
		{"example.com:443", "443"},
		{"example.com:8443", "8443"},
		{"example.com", "443"}, // no port at all: defaults to 443
	}
	for _, c := range cases {
		if got := port(c.hostport); got != c.want {
			t.Errorf("port(%q) = %q, want %q", c.hostport, got, c.want)
		}
	}
}

func TestStripDefaultHTTPSPort(t *testing.T) {
	cases := []struct {
		hostport string
		want     string
	}{
		{"example.com:443", "example.com"},
		{"example.com:8443", "example.com:8443"},
		{"not-a-hostport", "not-a-hostport"},
	}
	for _, c := range cases {
		if got := stripDefaultHTTPSPort(c.hostport); got != c.want {
			t.Errorf("stripDefaultHTTPSPort(%q) = %q, want %q", c.hostport, got, c.want)
		}
	}
}

func TestPassthroughTunnel_SplicesBytesUnmodified(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn) // echo whatever it receives
	}()

	clientSide, tunnelSide := net.Pipe()
	done := make(chan struct{})
	go func() {
		passthroughTunnel(tunnelSide, ln.Addr().String())
		close(done)
	}()

	_ = clientSide.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := clientSide.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 5)
	if _, err := io.ReadFull(clientSide, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "hello" {
		t.Fatalf("echoed = %q, want %q", buf, "hello")
	}

	clientSide.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("passthroughTunnel never returned after the client connection closed")
	}
}

// TestHandleConnect_TerminatesTLSAndForwards is the full integration test for
// the actual MITM behavior: a client CONNECTs, the engine hijacks the
// connection, completes a TLS handshake using a leaf certificate minted on
// the fly for the requested SNI (signed by our own freshly generated root
// CA), and a plain HTTP/1.1 request sent over that TLS connection is routed
// through the same Forward pipeline as any other request.
func TestHandleConnect_TerminatesTLSAndForwards(t *testing.T) {
	dir := t.TempDir()
	certAuthority, err := ca.LoadOrCreate(filepath.Join(dir, "ca-cert.pem"), filepath.Join(dir, "ca-key.pem"))
	if err != nil {
		t.Fatalf("generate CA: %v", err)
	}

	var capturedURL string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedURL = r.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("hello over TLS")),
		}, nil
	})
	forwarder := proxyuc.NewForwarder(&fakeHistory{}, fakeBroadcaster{}, proxyuc.NewInterceptor(), proxyuc.NewResponseInterceptor(), transport)
	engine := NewEngine(certAuthority, forwarder)

	proxySrv := httptest.NewServer(http.HandlerFunc(engine.ServeHTTP))
	defer proxySrv.Close()

	rawConn, err := net.Dial("tcp", proxySrv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer rawConn.Close()
	_ = rawConn.SetDeadline(time.Now().Add(5 * time.Second))

	const sni = "leaf.example.internal"
	if _, err := io.WriteString(rawConn, "CONNECT "+sni+":443 HTTP/1.1\r\nHost: "+sni+":443\r\n\r\n"); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}

	reader := bufio.NewReader(rawConn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	if statusLine != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("CONNECT response line = %q", statusLine)
	}
	// The engine writes just the status line with no headers — drain the
	// blank line terminating it.
	if blank, err := reader.ReadString('\n'); err != nil || blank != "\r\n" {
		t.Fatalf("expected a trailing blank line after CONNECT's status line, got %q (err=%v)", blank, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certAuthority.CertPEM()) {
		t.Fatal("failed to load the generated root CA into a cert pool")
	}

	tlsConn := tls.Client(rawConn, &tls.Config{RootCAs: pool, ServerName: sni, MinVersion: tls.VersionTLS12})
	defer tlsConn.Close()
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("TLS handshake against the on-the-fly leaf cert failed: %v", err)
	}
	if got := tlsConn.ConnectionState().PeerCertificates[0].Subject.CommonName; got != sni {
		t.Fatalf("leaf cert CommonName = %q, want %q", got, sni)
	}

	if _, err := io.WriteString(tlsConn, "GET /hello HTTP/1.1\r\nHost: "+sni+"\r\n\r\n"); err != nil {
		t.Fatalf("write request over TLS: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(tlsConn), nil)
	if err != nil {
		t.Fatalf("read response over TLS: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "hello over TLS" {
		t.Fatalf("response = %d %q, want 200 %q", resp.StatusCode, body, "hello over TLS")
	}
	if capturedURL != "https://"+sni+"/hello" {
		t.Fatalf("the decrypted request reaching Forward had URL %q, want %q", capturedURL, "https://"+sni+"/hello")
	}
}
