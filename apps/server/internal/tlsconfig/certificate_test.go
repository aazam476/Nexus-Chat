package tlsconfig

import (
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig/tlsconfigtest"
)

func TestValidateAcceptsAFullChain(t *testing.T) {
	authority := tlsconfigtest.NewAuthority(t)
	leaf, key := authority.Issue(t, tlsconfigtest.DefaultLeaf())

	certificate, info, err := Validate(authority.FullChain(leaf), key, Checks{Domain: "chat.example.com", Now: tlsconfigtest.Now(), Roots: authority.Roots})

	require.NoError(t, err)
	assert.Len(t, certificate.Certificate, 2)
	assert.Equal(t, []string{"chat.example.com"}, info.Names)
	assert.Equal(t, "Test Intermediate", info.Issuer)
	assert.Equal(t, tlsconfigtest.Now().Add(90*24*time.Hour), info.NotAfter)
}

func TestValidateAcceptsWildcards(t *testing.T) {
	authority := tlsconfigtest.NewAuthority(t)
	options := tlsconfigtest.DefaultLeaf()
	options.Names = []string{"*.example.com"}
	leaf, key := authority.Issue(t, options)

	_, _, err := Validate(authority.FullChain(leaf), key, Checks{Domain: "chat.example.com", Now: tlsconfigtest.Now(), Roots: authority.Roots})

	assert.NoError(t, err)
}

func TestValidateRejectsProblems(t *testing.T) {
	authority := tlsconfigtest.NewAuthority(t)
	leaf, key := authority.Issue(t, tlsconfigtest.DefaultLeaf())
	_, otherKey := authority.Issue(t, tlsconfigtest.DefaultLeaf())
	expiredOptions := tlsconfigtest.DefaultLeaf()
	expiredOptions.NotAfter = tlsconfigtest.Now().Add(-time.Minute)
	expiredLeaf, expiredKey := authority.Issue(t, expiredOptions)
	futureOptions := tlsconfigtest.DefaultLeaf()
	futureOptions.NotBefore = tlsconfigtest.Now().Add(time.Hour)
	futureLeaf, futureKey := authority.Issue(t, futureOptions)
	brokenIntermediate := append(append([]byte{}, leaf...), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{1, 2, 3}})...)

	tests := []struct {
		name        string
		certificate []byte
		key         []byte
		domain      string
		want        error
	}{
		{"no domain", authority.FullChain(leaf), key, "", ErrNoDomain},
		{"wrong key", authority.FullChain(leaf), otherKey, "chat.example.com", ErrKeyMismatch},
		{"not pem", []byte("hello"), key, "chat.example.com", ErrKeyMismatch},
		{"expired", authority.FullChain(expiredLeaf), expiredKey, "chat.example.com", ErrExpired},
		{"not yet valid", authority.FullChain(futureLeaf), futureKey, "chat.example.com", ErrNotYetValid},
		{"other domain", authority.FullChain(leaf), key, "other.example.org", ErrWrongDomain},
		{"missing intermediate", leaf, key, "chat.example.com", ErrIncomplete},
		{"garbage intermediate", brokenIntermediate, key, "chat.example.com", ErrIncomplete},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := Validate(test.certificate, test.key, Checks{Domain: test.domain, Now: tlsconfigtest.Now(), Roots: authority.Roots})

			assert.ErrorIs(t, err, test.want)
		})
	}
}

func TestValidateRejectsUntrustedRoot(t *testing.T) {
	trusted := tlsconfigtest.NewAuthority(t)
	untrusted := tlsconfigtest.NewAuthority(t)
	leaf, key := untrusted.Issue(t, tlsconfigtest.DefaultLeaf())

	_, _, err := Validate(untrusted.FullChain(leaf), key, Checks{Domain: "chat.example.com", Now: tlsconfigtest.Now(), Roots: trusted.Roots})

	assert.ErrorIs(t, err, ErrIncomplete)
}
