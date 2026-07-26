// Package urlnorm holds the one normalization rule shared by every tool that
// accepts a user-typed target URL (the scanners, the spider, the port
// scanner's host field): people paste "example.com" as often as
// "https://example.com", so a bare scheme-less host is assumed to be HTTPS
// rather than rejected.
package urlnorm

import "strings"

// EnsureScheme trims raw and, if it has neither an "http://" nor "https://"
// prefix, assumes HTTPS. An empty (or all-whitespace) input is returned as
// "" so callers can still validate "target is required" themselves.
func EnsureScheme(raw string) string {
	target := strings.TrimSpace(raw)
	if target == "" {
		return ""
	}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	return "https://" + target
}
