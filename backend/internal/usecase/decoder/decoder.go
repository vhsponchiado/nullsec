// Package codec implements the transforms behind the Decoder tool: a set of
// reversible encodings (Base64, Base32, hex, URL percent-encoding, HTML
// entities, gzip) plus one-way hashes, and a Burp-style "smart decode" that
// auto-detects and peels off encoding layers one at a time.
package decoder

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"  //nolint:gosec // G501: Decoder tool computes MD5 on user-supplied text as a utility, never for auth/integrity
	"crypto/sha1" //nolint:gosec // G505: same as above, for SHA-1
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
)

type Operation struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Hash operations are one-way: only Encode (hashing) is offered, never Decode.
	HashOnly bool `json:"hash_only"`
}

var Operations = []Operation{
	{Key: "base64", Label: "Base64"},
	{Key: "base64url", Label: "Base64 (URL-safe)"},
	{Key: "base32", Label: "Base32"},
	{Key: "hex", Label: "Hex"},
	{Key: "url", Label: "URL"},
	{Key: "html", Label: "HTML"},
	{Key: "gzip", Label: "Gzip (base64)"},
	{Key: "md5", Label: "MD5", HashOnly: true},
	{Key: "sha1", Label: "SHA-1", HashOnly: true},
	{Key: "sha256", Label: "SHA-256", HashOnly: true},
	{Key: "sha512", Label: "SHA-512", HashOnly: true},
}

func labelFor(op string) string {
	for _, o := range Operations {
		if o.Key == op {
			return o.Label
		}
	}
	return op
}

func Encode(op, input string) (string, error) {
	switch op {
	case "base64":
		return base64.StdEncoding.EncodeToString([]byte(input)), nil
	case "base64url":
		return base64.URLEncoding.EncodeToString([]byte(input)), nil
	case "base32":
		return base32.StdEncoding.EncodeToString([]byte(input)), nil
	case "hex":
		return hex.EncodeToString([]byte(input)), nil
	case "url":
		return url.QueryEscape(input), nil
	case "html":
		return html.EscapeString(input), nil
	case "gzip":
		// Compressed bytes aren't valid UTF-8 in general, and our transport
		// (JSON) can't carry arbitrary bytes in a string — so "gzip encode"
		// here means "gzip-compress, then base64 the result", matching what
		// most text-based gzip tools show as the encoded text.
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write([]byte(input)); err != nil {
			return "", err
		}
		if err := gw.Close(); err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
	case "md5":
		sum := md5.Sum([]byte(input)) //nolint:gosec // G401: see import above
		return hex.EncodeToString(sum[:]), nil
	case "sha1":
		sum := sha1.Sum([]byte(input)) //nolint:gosec // G401: see import above
		return hex.EncodeToString(sum[:]), nil
	case "sha256":
		sum := sha256.Sum256([]byte(input))
		return hex.EncodeToString(sum[:]), nil
	case "sha512":
		sum := sha512.Sum512([]byte(input))
		return hex.EncodeToString(sum[:]), nil
	default:
		return "", fmt.Errorf("unknown operation: %s", op)
	}
}

func Decode(op, input string) (string, error) {
	switch op {
	case "base64":
		out, err := base64.StdEncoding.DecodeString(input)
		return string(out), err
	case "base64url":
		out, err := base64.URLEncoding.DecodeString(input)
		return string(out), err
	case "base32":
		out, err := base32.StdEncoding.DecodeString(input)
		return string(out), err
	case "hex":
		out, err := hex.DecodeString(input)
		return string(out), err
	case "url":
		return url.QueryUnescape(input)
	case "html":
		return html.UnescapeString(input), nil
	case "gzip":
		raw, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			return "", fmt.Errorf("expected base64-encoded gzip data: %w", err)
		}
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		defer gr.Close()
		out, err := io.ReadAll(gr)
		return string(out), err
	case "md5", "sha1", "sha256", "sha512":
		return "", fmt.Errorf("%s is a one-way hash — it cannot be decoded", op)
	default:
		return "", fmt.Errorf("unknown operation: %s", op)
	}
}

type SmartStep struct {
	Op     string `json:"op"`
	Label  string `json:"label"`
	Output string `json:"output"`
}

const maxSmartSteps = 8

var (
	htmlEntityRe = regexp.MustCompile(`&(#\d+|#x[0-9A-Fa-f]+|[a-zA-Z]+);`)
	percentRe    = regexp.MustCompile(`%[0-9A-Fa-f]{2}`)
	hexRe        = regexp.MustCompile(`^[0-9A-Fa-f]+$`)
	base64Re     = regexp.MustCompile(`^[A-Za-z0-9+/]+={0,2}$`)
	base64URLRe  = regexp.MustCompile(`^[A-Za-z0-9_-]+={0,2}$`)
	base32Re     = regexp.MustCompile(`^[A-Z2-7]+=*$`)
)

// looksLikeGzip reports whether decoding s as base64 yields gzip's magic
// bytes — checked before generic base64 detection so a gzip+base64 blob
// decodes as one "Gzip" step instead of stopping one layer early at "Base64".
func looksLikeGzip(s string) bool {
	raw, err := base64.StdEncoding.DecodeString(s)
	return err == nil && len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b
}

// detect picks the single most likely encoding of s, checking the most
// specific/least ambiguous signals first.
func detect(s string) (string, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", false
	}
	if htmlEntityRe.MatchString(s) {
		return "html", true
	}
	if percentRe.MatchString(s) {
		return "url", true
	}
	if looksLikeGzip(trimmed) {
		return "gzip", true
	}
	if len(trimmed) >= 4 && len(trimmed)%4 == 0 && base64Re.MatchString(trimmed) {
		return "base64", true
	}
	if len(trimmed) >= 4 && len(trimmed)%4 == 0 && strings.ContainsAny(trimmed, "-_") && base64URLRe.MatchString(trimmed) {
		return "base64url", true
	}
	if len(trimmed) >= 8 && len(trimmed)%8 == 0 && base32Re.MatchString(trimmed) {
		return "base32", true
	}
	if len(trimmed) >= 2 && len(trimmed)%2 == 0 && hexRe.MatchString(trimmed) {
		return "hex", true
	}
	return "", false
}

// SmartDecode repeatedly detects and peels off one encoding layer at a time
// (like Burp Decoder's "Smart decode"), stopping when nothing more is
// detected, decoding fails, or the output stops changing.
func SmartDecode(input string) []SmartStep {
	current := input
	seen := map[string]bool{current: true}
	steps := []SmartStep{}

	for i := 0; i < maxSmartSteps; i++ {
		op, ok := detect(current)
		if !ok {
			break
		}
		out, err := Decode(op, current)
		if err != nil || out == current || seen[out] {
			break
		}
		steps = append(steps, SmartStep{Op: op, Label: labelFor(op), Output: out})
		seen[out] = true
		current = out
	}
	return steps
}
