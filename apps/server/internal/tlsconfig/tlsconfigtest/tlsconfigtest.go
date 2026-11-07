// Package tlsconfigtest issues certificates from a throwaway certificate
// authority for tests: a root, an intermediate, and server certificates,
// shaped like those from a real public CA.
package tlsconfigtest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Now returns the fixed "now" of every certificate test. Tests use it as
// their clock, so certificate dates never depend on the real time.
func Now() time.Time {
	return time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
}

// Authority is a root CA plus an intermediate, like a real public CA.
type Authority struct {
	// Roots trusts only this authority's root.
	Roots           *x509.CertPool
	intermediate    *x509.Certificate
	intermediateKey *ecdsa.PrivateKey
	intermediatePEM []byte
}

func newKey(t testing.TB) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

// NewAuthority creates a fresh root and intermediate.
func NewAuthority(t testing.TB) Authority {
	t.Helper()
	rootKey := newKey(t)
	rootTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test Root"},
		NotBefore: Now().Add(-time.Hour), NotAfter: Now().Add(10 * 365 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)
	root, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	intermediateKey := newKey(t)
	intermediateTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "Test Intermediate"},
		NotBefore: Now().Add(-time.Hour), NotAfter: Now().Add(5 * 365 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	intermediateDER, err := x509.CreateCertificate(rand.Reader, intermediateTemplate, root, &intermediateKey.PublicKey, rootKey)
	require.NoError(t, err)
	intermediate, err := x509.ParseCertificate(intermediateDER)
	require.NoError(t, err)

	roots := x509.NewCertPool()
	roots.AddCert(root)
	return Authority{Roots: roots, intermediate: intermediate, intermediateKey: intermediateKey,
		intermediatePEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intermediateDER})}
}

// LeafOptions describes a server certificate to issue.
type LeafOptions struct {
	Names     []string
	NotBefore time.Time
	NotAfter  time.Time
}

// DefaultLeaf is a certificate for chat.example.com valid from an hour
// before Now until 90 days after it.
func DefaultLeaf() LeafOptions {
	return LeafOptions{Names: []string{"chat.example.com"}, NotBefore: Now().Add(-time.Hour), NotAfter: Now().Add(90 * 24 * time.Hour)}
}

// Issue returns a leaf certificate PEM (without the chain) and its key PEM.
func (authority Authority) Issue(t testing.TB, options LeafOptions) ([]byte, []byte) {
	t.Helper()
	key := newKey(t)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: options.Names[0]},
		DNSNames: options.Names, NotBefore: options.NotBefore, NotAfter: options.NotAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, authority.intermediate, &key.PublicKey, authority.intermediateKey)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// FullChain returns leaf followed by the intermediate, as admins should
// install it.
func (authority Authority) FullChain(leaf []byte) []byte {
	return append(append([]byte{}, leaf...), authority.intermediatePEM...)
}
