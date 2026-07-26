// Package certificate is the "manage the MITM root CA" use case: download
// the current certificate, or regenerate it (which invalidates every leaf
// certificate issued so far). The only implementation of CertAuthority is
// infra/ca.CA, but this use case doesn't know that.
package certificate

// CertAuthority is the root CA this use case manages.
type CertAuthority interface {
	CertPEM() []byte
	Regenerate() error
}

type UseCase struct {
	ca CertAuthority
}

func New(ca CertAuthority) *UseCase {
	return &UseCase{ca: ca}
}

// DownloadCA returns the current CA certificate, PEM-encoded.
func (u *UseCase) DownloadCA() []byte {
	return u.ca.CertPEM()
}

// RegenerateCA replaces the CA with a brand new key pair, invalidating
// every certificate issued under the old one.
func (u *UseCase) RegenerateCA() error {
	return u.ca.Regenerate()
}
