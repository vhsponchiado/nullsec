// Package wordlist ships built-in wordlists for use as Intruder payload
// sources: a password list, a username list, and a list of username:password
// pairs. The pairs are meant to be split across two positions (usernames
// into one, passwords into the other) and tested together with Intruder's
// Pitchfork attack — that reproduces the original pairs; Cluster bomb would
// instead combine every username with every password, losing the pairing.
//
// This is a small, curated lab/CTF wordlist provided by the project's
// maintainer — not sourced from a public breach-data corpus.
package wordlist

import (
	_ "embed"
	"strings"
)

//go:embed passwords.txt
var rawPasswords string

//go:embed usernames.txt
var rawUsernames string

//go:embed credentials.txt
var rawCredentials string

// Passwords is the built-in password wordlist.
var Passwords = splitLines(rawPasswords)

// Usernames is the built-in username wordlist.
var Usernames = splitLines(rawUsernames)

// CredentialPair is one username:password pair from the credentials wordlist.
type CredentialPair struct {
	Username string
	Password string
}

// Credentials is the built-in list of username:password pairs.
var Credentials = parseCredentials(rawCredentials)

// CredentialUsernames and CredentialPasswords return the two halves of
// Credentials, in the same order — selecting one for each of two marked
// Intruder positions and running a Pitchfork attack (one payload set per
// position, advancing together) tests the original pairs; picking these for
// a Cluster bomb attack instead tries every combination of the two lists,
// not just the real pairs.
func CredentialUsernames() []string {
	out := make([]string, len(Credentials))
	for i, c := range Credentials {
		out[i] = c.Username
	}
	return out
}

func CredentialPasswords() []string {
	out := make([]string, len(Credentials))
	for i, c := range Credentials {
		out[i] = c.Password
	}
	return out
}

func splitLines(raw string) []string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func parseCredentials(raw string) []CredentialPair {
	lines := splitLines(raw)
	out := make([]CredentialPair, 0, len(lines))
	for _, l := range lines {
		user, pass, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		out = append(out, CredentialPair{Username: user, Password: pass})
	}
	return out
}

// Adapter implements usecase/intruder.Wordlists.
type Adapter struct{}

func (Adapter) Passwords() []string           { return Passwords }
func (Adapter) Usernames() []string           { return Usernames }
func (Adapter) CredentialUsernames() []string { return CredentialUsernames() }
func (Adapter) CredentialPasswords() []string { return CredentialPasswords() }
