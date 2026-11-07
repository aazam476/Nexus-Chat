package setup

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"strings"
)

// newToken returns a fresh setup token for display, grouped for easier
// typing (for example "ABCD-EFGH-IJKL-MNOP-QRST-UVWX-YZ"), and its hash.
// rand.Text gives 26 base32 characters: 128 bits of randomness.
func newToken() (string, [sha256.Size]byte) {
	raw := rand.Text()
	var groups []string
	for start := 0; start < len(raw); start += 4 {
		groups = append(groups, raw[start:min(start+4, len(raw))])
	}
	return strings.Join(groups, "-"), hashToken(raw)
}

// hashToken hashes a token after normalizing it, so "abcd-efgh" and
// "ABCDEFGH" are the same token. Only the hash is kept in memory, never the
// token itself.
func hashToken(token string) [sha256.Size]byte {
	normalized := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(token)))
	return sha256.Sum256([]byte(normalized))
}

// tokenMatches compares in constant time, so response timing reveals
// nothing about how much of a guess was right.
func tokenMatches(candidate string, want [sha256.Size]byte) bool {
	got := hashToken(candidate)
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}
