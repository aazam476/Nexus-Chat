package session

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestRequireRecent(t *testing.T) {
	loggedIn := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	principal := Principal{AuthenticatedAt: loggedIn}

	require.NoError(t, principal.RequireRecent(loggedIn))
	require.NoError(t, principal.RequireRecent(loggedIn.Add(RecentAuthentication)))
	require.ErrorIs(t, principal.RequireRecent(loggedIn.Add(RecentAuthentication+time.Second)), ErrReauthenticationRequired)
	require.ErrorIs(t, Principal{}.RequireRecent(loggedIn), ErrReauthenticationRequired, "an unknown time is not recent")
}
