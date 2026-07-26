package wordlist

import "testing"

func TestSplitLines(t *testing.T) {
	got := splitLines("admin\n  test  \n\nroot\n")
	want := []string{"admin", "test", "root"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestParseCredentials(t *testing.T) {
	pairs := parseCredentials("admin:admin123\nroot:toor\nmalformed-line\nuser:pa:ss")
	if len(pairs) != 3 {
		t.Fatalf("expected 3 valid pairs (malformed-line skipped), got %d: %v", len(pairs), pairs)
	}
	if pairs[0] != (CredentialPair{Username: "admin", Password: "admin123"}) {
		t.Errorf("got %+v, want admin:admin123", pairs[0])
	}
	if pairs[1] != (CredentialPair{Username: "root", Password: "toor"}) {
		t.Errorf("got %+v, want root:toor", pairs[1])
	}
	// strings.Cut splits on the FIRST colon, so a value containing one
	// itself (e.g. a password with a colon) stays intact in Password.
	if pairs[2] != (CredentialPair{Username: "user", Password: "pa:ss"}) {
		t.Errorf("got %+v, want user:pa:ss split on first colon", pairs[2])
	}
}

func TestCredentialUsernamesAndPasswordsStayPaired(t *testing.T) {
	if len(Credentials) == 0 {
		t.Fatal("expected the embedded credentials.txt to parse at least one pair")
	}
	usernames := CredentialUsernames()
	passwords := CredentialPasswords()
	if len(usernames) != len(Credentials) || len(passwords) != len(Credentials) {
		t.Fatalf("got %d usernames, %d passwords, want %d (matching Credentials)", len(usernames), len(passwords), len(Credentials))
	}
	for i, c := range Credentials {
		if usernames[i] != c.Username || passwords[i] != c.Password {
			t.Fatalf("index %d: usernames/passwords diverged from Credentials pair %+v", i, c)
		}
	}
}
