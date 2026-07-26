package urlnorm

import "testing"

func TestEnsureScheme(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare host", "example.com", "https://example.com"},
		{"bare host with path", "example.com/path?q=1", "https://example.com/path?q=1"},
		{"already https", "https://example.com", "https://example.com"},
		{"already http", "http://example.com", "http://example.com"},
		{"surrounding whitespace", "  example.com  ", "https://example.com"},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EnsureScheme(tc.in); got != tc.want {
				t.Errorf("EnsureScheme(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
