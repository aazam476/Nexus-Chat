package oidc

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsernameBase(t *testing.T) {
	tests := map[string]Claims{
		"ada.lovelace":                     {PreferredUsername: "Ada.Lovelace"},
		"j_rgen_m_ller":                    {PreferredUsername: "Jürgen Müller"},
		"from_email":                       {Email: "from-email@example.com"},
		"user":                             {PreferredUsername: "__"},
		"x23":                              {PreferredUsername: ".x23."},
		strings.Repeat("a", 32):            {PreferredUsername: strings.Repeat("a", 40)},
		"user ":                            {},
		"preferred_wins_over_email_handle": {PreferredUsername: "preferred_wins_over_email_handle", Email: "other@example.com"},
	}
	for want, claims := range tests {
		assert.Equal(t, strings.TrimSpace(want), usernameBase(claims), "%+v", claims)
	}
}

func TestPickUsernameAddsASuffixWhenTaken(t *testing.T) {
	fixture := newSignInFixture(t)
	fixture.store.usernames["ada"] = true

	username, err := fixture.service.pickUsername(t.Context(), Claims{PreferredUsername: "ada"})

	require.NoError(t, err)
	assert.Regexp(t, `^ada_[a-z2-7]{4}$`, username)
}

func TestPickUsernameGivesUpEventually(t *testing.T) {
	fixture := newSignInFixture(t)
	takenAll := &alwaysTaken{fakeStore: fixture.store}
	fixture.service.dependencies.Store = takenAll

	_, err := fixture.service.pickUsername(t.Context(), Claims{PreferredUsername: strings.Repeat("b", 32)})

	require.ErrorIs(t, err, errNoFreeUsername)
	assert.Equal(t, usernameAttempts, takenAll.checks)
	for _, candidate := range takenAll.candidates[1:] {
		assert.Len(t, candidate, 32, "a suffix never makes the name too long")
	}
}

// alwaysTaken reports every username as taken.
type alwaysTaken struct {
	*fakeStore
	checks     int
	candidates []string
}

func (fake *alwaysTaken) UsernameExists(_ context.Context, username string) (bool, error) {
	fake.checks++
	fake.candidates = append(fake.candidates, username)
	return true, nil
}

func TestDisplayName(t *testing.T) {
	assert.Equal(t, "Ada Lovelace", displayName("  Ada \n Lovelace ", "ada"))
	assert.Equal(t, "ada", displayName("", "ada"))
	assert.Equal(t, "ada", displayName("\xff\xfe", "ada"), "invalid UTF-8")
	assert.Equal(t, strings.Repeat("é", 32), displayName(strings.Repeat("é", 40), "ada"))
}
