package mitm

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// tunnelIdleTimeout bounds how long we wait for the next request/response on
// a decrypted tunnel or a raw (non-443) passthrough before giving up. Without
// it a half-open or abandoned connection (browsers open many and let idle
// ones linger) holds a goroutine + file descriptor forever.
const tunnelIdleTimeout = 2 * time.Minute

// handleConnect terminates the CONNECT tunnel itself (MITM) instead of
// blindly splicing bytes between client and destination: it hijacks the raw
// connection, completes a TLS handshake with the client using a leaf
// certificate minted on the fly for the requested SNI hostname, and then
// reads plaintext HTTP/1.1 requests off that TLS connection — each one is
// run through the same Forward pipeline used for plain HTTP, so intercept,
// history and Repeater all work identically for HTTPS traffic.
func (e *Engine) handleConnect(w http.ResponseWriter, r *http.Request) {
	if e.ca == nil {
		http.Error(w, "HTTPS interception not configured", http.StatusNotImplemented)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("hijack failed for CONNECT %q: %v", r.Host, err) //nolint:gosec // G706: %q quotes/escapes r.Host, so a newline in it can't forge a log line
		return
	}
	defer clientConn.Close()

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	// Only decrypt-and-parse-as-HTTP on the standard HTTPS port. CONNECT to
	// any other port (e.g. Google's proprietary push-messaging channel on
	// 5228) is almost certainly not HTTP/1.1 over TLS, so trying to MITM it
	// would just fail to parse forever; tunnel those transparently instead.
	if port(r.Host) != "443" {
		passthroughTunnel(clientConn, r.Host)
		return
	}

	tlsConn := tls.Server(clientConn, &tls.Config{
		GetCertificate: e.ca.GetCertificate,
		// Force HTTP/1.1: our tunnel reader only understands HTTP/1.1 framing,
		// so we must not let the client negotiate HTTP/2 via ALPN.
		NextProtos: []string{"http/1.1"},
	})
	defer tlsConn.Close()
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("TLS handshake with client failed for %q: %v", r.Host, err) //nolint:gosec // G706: %q quotes/escapes r.Host, see comment above
		return
	}

	// Browsers always include the port in a CONNECT target (e.g.
	// "example.com:443"), but a normal HTTPS request never shows ":443"
	// explicitly — it's the implicit default. Carrying it through verbatim
	// produces URLs like "https://example.com:443/path" that some servers'
	// Host-header validation (CSRF checks, WAFs, signed-request schemes)
	// reject even though it's technically equivalent.
	connHost := stripDefaultHTTPSPort(r.Host)
	reader := bufio.NewReader(tlsConn)

	for {
		_ = tlsConn.SetDeadline(time.Now().Add(tunnelIdleTimeout))

		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("failed to read request over TLS tunnel to %q: %v", connHost, err) //nolint:gosec // G706: %q quotes/escapes connHost, see comment above
			}
			return
		}

		req.URL.Scheme = "https"
		if req.URL.Host == "" {
			req.URL.Host = connHost
		}
		if req.Host == "" {
			req.Host = connHost
		}

		keepAlive := req.ProtoAtLeast(1, 1) && !strings.EqualFold(req.Header.Get("Connection"), "close")

		e.serveOverTunnel(tlsConn, req)

		if !keepAlive {
			return
		}
	}
}

// port extracts the port from a CONNECT target ("host:port"), defaulting to
// 443 if none is present.
func port(hostport string) string {
	_, p, err := net.SplitHostPort(hostport)
	if err != nil {
		return "443"
	}
	return p
}

// stripDefaultHTTPSPort removes a trailing ":443" from a "host:port" string
// (the implicit default for HTTPS), leaving non-standard ports untouched.
func stripDefaultHTTPSPort(hostport string) string {
	host, p, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	if p == "443" {
		return host
	}
	return hostport
}

// passthroughTunnel splices bytes between the client and the destination
// without attempting to decrypt or parse anything — used for CONNECT targets
// that aren't plain HTTPS (arbitrary TLS-ish or fully custom protocols).
func passthroughTunnel(clientConn net.Conn, hostport string) {
	// A proxy's entire purpose is to connect wherever its client asks —
	// hostport comes from the client's own CONNECT request, same as every
	// other tunnel this proxy opens. There's no privilege boundary being
	// crossed here for gosec's SSRF check to be warning about.
	destConn, err := net.DialTimeout("tcp", hostport, 10*time.Second) //nolint:gosec // G704: intentional, see comment above
	if err != nil {
		log.Printf("failed to dial passthrough target %q: %v", hostport, err) //nolint:gosec // G706: %q quotes/escapes hostport
		return
	}
	defer destConn.Close()

	done := make(chan struct{}, 2)
	copyLoop := func(dst, src net.Conn) {
		_ = src.SetDeadline(time.Now().Add(tunnelIdleTimeout))
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}
	go copyLoop(destConn, clientConn)
	go copyLoop(clientConn, destConn)
	<-done
}

// serveOverTunnel runs a decrypted request through the shared Forward
// pipeline and writes an HTTP/1.1 response back over the TLS tunnel.
func (e *Engine) serveOverTunnel(conn net.Conn, r *http.Request) {
	result := e.forwarder.Forward(r, false)

	var resp *http.Response
	switch {
	case result.Dropped:
		resp = textResponse(r, http.StatusBadGateway, "request dropped by proxy")
	case result.TimedOut:
		resp = textResponse(r, http.StatusGatewayTimeout, "request timed out waiting for intercept decision")
	case result.Err != nil:
		resp = textResponse(r, http.StatusBadGateway, "proxy error: "+result.Err.Error())
	default:
		resp = &http.Response{
			StatusCode:    result.Status,
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        result.Headers,
			Body:          io.NopCloser(bytes.NewReader(result.Body)),
			ContentLength: int64(len(result.Body)),
			Request:       r,
		}
	}

	defer resp.Body.Close()
	if err := resp.Write(conn); err != nil {
		log.Printf("failed to write response over TLS tunnel for %q: %v", r.URL, err) //nolint:gosec // G706: %q quotes/escapes r.URL's string form
	}
}

func textResponse(r *http.Request, status int, msg string) *http.Response {
	body := []byte(msg)
	return &http.Response{
		StatusCode: status,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Content-Type":   []string{"text/plain; charset=utf-8"},
			"Content-Length": []string{fmt.Sprint(len(body))},
		},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}
}
