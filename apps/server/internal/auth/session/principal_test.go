package session

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPrincipalRoundTrip(t *testing.T) {
	principal := Principal{UserID: uuid.New(), SessionID: uuid.New()}

	got, ok := PrincipalFrom(WithPrincipal(context.Background(), principal))

	assert.True(t, ok)
	assert.Equal(t, principal, got)
	_, ok = PrincipalFrom(context.Background())
	assert.False(t, ok)
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		header string
		want   string
		ok     bool
	}{
		{"Bearer nxs_abc", "nxs_abc", true},
		{"bearer nxs_abc", "nxs_abc", true},
		{"Bearer   nxs_abc  ", "nxs_abc", true},
		{"Basic dXNlcjpwYXNz", "", false},
		{"Bearer", "", false},
		{"Bearer    ", "", false},
		{"", "", false},
	}
	for _, test := range tests {
		token, ok := BearerToken(test.header)
		assert.Equal(t, test.ok, ok, test.header)
		assert.Equal(t, test.want, token, test.header)
	}
}
