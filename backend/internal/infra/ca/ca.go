// Package ca implements a self-signed root Certificate Authority used to
// sign per-domain "leaf" certificates on the fly, so the proxy can terminate
// TLS for intercepted HTTPS connections (classic MITM technique). The root
// CA is generated once and persisted to disk; leaf certificates are cached
// in memory for the lifetime of the process.
package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CA struct {
	mu       sync.Mutex
	cert     *x509.Certificate
	key      *ecdsa.PrivateKey
	certPEM  []byte
	certPath string
	keyPath  string
	leaves   map[string]*tls.Certificate
}

// LoadOrCreate loads an existing CA key pair from disk, or generates a new
// one and persists it if the files don't exist yet.
func LoadOrCreate(certPath, keyPath string) (*CA, error) {
	c := &CA{certPath: certPath, keyPath: keyPath, leaves: make(map[string]*tls.Certificate)}

	certBytes, certErr := os.ReadFile(certPath)
	keyBytes, keyErr := os.ReadFile(keyPath)

	if certErr == nil && keyErr == nil {
		if err := c.load(certBytes, keyBytes); err != nil {
			return nil, err
		}
		return c, nil
	}

	if err := c.generate(); err != nil {
		return nil, err
	}
	if err := c.persist(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *CA) load(certPEMBytes, keyPEMBytes []byte) error {
	certBlock, _ := pem.Decode(certPEMBytes)
	if certBlock == nil {
		return fmt.Errorf("invalid CA certificate PEM at %s", c.certPath)
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse CA certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEMBytes)
	if keyBlock == nil {
		return fmt.Errorf("invalid CA key PEM at %s", c.keyPath)
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse CA key: %w", err)
	}

	c.cert = cert
	c.key = key
	c.certPEM = certPEMBytes
	return nil
}

func (c *CA) generate() error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Nullsec Proxy CA",
			Organization: []string{"Nullsec"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}

	c.cert = cert
	c.key = key
	c.certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return nil
}

func (c *CA) persist() error {
	if err := os.MkdirAll(filepath.Dir(c.certPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.keyPath), 0o755); err != nil {
		return err
	}
	// The CA certificate is meant to be installed into browser/OS trust
	// stores and downloaded over HTTP (see api.downloadCACert) — 0644 (world
	// readable, owner writable) is intentional; it's the private key below
	// that needs to stay locked down.
	if err := os.WriteFile(c.certPath, c.certPEM, 0o644); err != nil { //nolint:gosec // G306: public cert, see comment above
		return err
	}

	keyBytes, err := x509.MarshalECPrivateKey(c.key)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	return os.WriteFile(c.keyPath, keyPEM, 0o600)
}

// CertPEM returns the current root CA certificate, PEM-encoded.
func (c *CA) CertPEM() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.certPEM
}

// Regenerate replaces the CA with a brand new key pair (persisted to disk,
// overwriting the previous one) and clears all cached leaf certificates.
// Any certificate issued under the old CA stops validating immediately.
func (c *CA) Regenerate() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.generate(); err != nil {
		return err
	}
	if err := c.persist(); err != nil {
		return err
	}
	c.leaves = make(map[string]*tls.Certificate)
	return nil
}

// GetCertificate implements the signature expected by tls.Config.GetCertificate:
// it returns a leaf certificate for the SNI hostname the TLS client requested,
// generating (and caching) a new one signed by the CA on first use.
func (c *CA) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := hello.ServerName
	if host == "" {
		return nil, fmt.Errorf("no SNI hostname provided by client")
	}
	return c.leafFor(host)
}

func (c *CA) leafFor(host string) (*tls.Certificate, error) {
	c.mu.Lock()
	if leaf, ok := c.leaves[host]; ok {
		c.mu.Unlock()
		return leaf, nil
	}
	caCert, caKey := c.cert, c.key
	c.mu.Unlock()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}

	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	leaf := &tls.Certificate{
		Certificate: [][]byte{der, caCert.Raw},
		PrivateKey:  key,
	}

	c.mu.Lock()
	c.leaves[host] = leaf
	c.mu.Unlock()

	return leaf, nil
}
