package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"time"
)

// Errors explaining why a certificate cannot be used. The Admin panel shows
// them to the admin, so they say what to fix.
var (
	ErrNoDomain    = errors.New("set the instance domain before enabling HTTPS")
	ErrKeyMismatch = errors.New("the private key does not match the certificate")
	ErrNotYetValid = errors.New("the certificate is not valid yet")
	ErrExpired     = errors.New("the certificate has expired")
	ErrWrongDomain = errors.New("the certificate does not cover the instance domain")
	ErrIncomplete  = errors.New("the certificate chain is incomplete or not trusted; include the intermediate certificates")
)

// Info summarizes a validated certificate for logs and the Admin panel.
type Info struct {
	Names    []string
	Issuer   string
	NotAfter time.Time
}

// Checks are the inputs to Validate besides the PEM data.
type Checks struct {
	// Domain is the instance's public domain. Required.
	Domain string
	// Now is the current time.
	Now time.Time
	// Roots are the trusted root certificates. nil means the system's
	// roots; tests pass their own test CA.
	Roots *x509.CertPool
}

// Validate parses a PEM certificate chain and private key and checks that
// they are usable for checks.Domain.
func Validate(certificatePEM []byte, keyPEM []byte, checks Checks) (*tls.Certificate, Info, error) {
	if checks.Domain == "" {
		return nil, Info{}, ErrNoDomain
	}
	pair, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		return nil, Info{}, fmt.Errorf("%w: %w", ErrKeyMismatch, err)
	}
	// X509KeyPair fills in Leaf since Go 1.23.
	leaf := pair.Leaf
	if err := checkValidity(leaf, checks.Now); err != nil {
		return nil, Info{}, err
	}
	if err := leaf.VerifyHostname(checks.Domain); err != nil {
		return nil, Info{}, fmt.Errorf("%w %q (it covers %v)", ErrWrongDomain, checks.Domain, leaf.DNSNames)
	}
	if err := verifyChain(pair, checks); err != nil {
		return nil, Info{}, err
	}
	return &pair, Info{Names: leaf.DNSNames, Issuer: leaf.Issuer.CommonName, NotAfter: leaf.NotAfter}, nil
}

func checkValidity(leaf *x509.Certificate, now time.Time) error {
	if now.Before(leaf.NotBefore) {
		return fmt.Errorf("%w (valid from %s)", ErrNotYetValid, leaf.NotBefore.Format(time.RFC3339))
	}
	if now.After(leaf.NotAfter) {
		return fmt.Errorf("%w (expired %s)", ErrExpired, leaf.NotAfter.Format(time.RFC3339))
	}
	return nil
}

// verifyChain checks that the leaf chains to a trusted root using only the
// intermediates in the file. Browsers often fetch missing intermediates
// themselves, but apps and other servers do not, so an incomplete chain
// would break federation and mobile clients.
func verifyChain(pair tls.Certificate, checks Checks) error {
	intermediates := x509.NewCertPool()
	for _, der := range pair.Certificate[1:] {
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrIncomplete, err)
		}
		intermediates.AddCert(certificate)
	}
	_, err := pair.Leaf.Verify(x509.VerifyOptions{
		DNSName:       checks.Domain,
		Intermediates: intermediates,
		Roots:         checks.Roots,
		CurrentTime:   checks.Now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrIncomplete, err)
	}
	return nil
}
