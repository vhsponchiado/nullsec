package entity

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// UpstreamProxyType selects how outbound tools (Repeater, Intruder, Port
// Scanner, the Subdomain Finder's crt.sh lookup) reach their targets.
type UpstreamProxyType string

const (
	UpstreamNone   UpstreamProxyType = "none"
	UpstreamSOCKS5 UpstreamProxyType = "socks5"
	UpstreamHTTP   UpstreamProxyType = "http"
)

// UpstreamProxyConfig is a SOCKS5 or HTTP(S) forward proxy the user
// controls — a local Tor daemon, a VPN's SOCKS listener, a client-provided
// jump box — routing every outbound tool's traffic through infrastructure
// they're entitled to use, rather than dialing targets directly from this
// machine. It doesn't make unauthorized use of those tools any safer: it
// only changes which IP the traffic originates from.
type UpstreamProxyConfig struct {
	Type     UpstreamProxyType `json:"type"`
	Address  string            `json:"address"`
	Username string            `json:"username,omitempty"`
	Password string            `json:"password,omitempty"`
}

// Validate checks the config's own invariants (a SOCKS5/HTTP proxy needs a
// well-formed host:port address) — it can't check reachability, only shape.
func (c UpstreamProxyConfig) Validate() error {
	switch c.Type {
	case UpstreamNone, "":
		return nil
	case UpstreamSOCKS5, UpstreamHTTP:
		if strings.TrimSpace(c.Address) == "" {
			return errors.New("address is required")
		}
		if _, _, err := net.SplitHostPort(c.Address); err != nil {
			return fmt.Errorf("invalid address %q: expected host:port", c.Address)
		}
		return nil
	default:
		return fmt.Errorf("unknown proxy type: %s", c.Type)
	}
}
