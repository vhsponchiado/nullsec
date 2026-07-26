package certificate

import (
	"errors"
	"testing"
)

type fakeCA struct {
	pem           []byte
	regenerateErr error
	regenerated   bool
}

func (f *fakeCA) CertPEM() []byte { return f.pem }
func (f *fakeCA) Regenerate() error {
	f.regenerated = true
	return f.regenerateErr
}

func TestDownloadCA(t *testing.T) {
	ca := &fakeCA{pem: []byte("-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----")}
	uc := New(ca)

	if got := uc.DownloadCA(); string(got) != string(ca.pem) {
		t.Fatalf("DownloadCA() = %q, want %q", got, ca.pem)
	}
}

func TestRegenerateCA_Success(t *testing.T) {
	ca := &fakeCA{}
	uc := New(ca)

	if err := uc.RegenerateCA(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ca.regenerated {
		t.Fatal("RegenerateCA should have called the underlying CA's Regenerate")
	}
}

func TestRegenerateCA_PropagatesError(t *testing.T) {
	wantErr := errors.New("disk full")
	ca := &fakeCA{regenerateErr: wantErr}
	uc := New(ca)

	if err := uc.RegenerateCA(); !errors.Is(err, wantErr) {
		t.Fatalf("RegenerateCA() error = %v, want %v", err, wantErr)
	}
}
