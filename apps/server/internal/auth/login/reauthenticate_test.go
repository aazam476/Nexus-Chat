package login

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

func reauthFixture(change func(*Dependencies)) (fixture, *[]session.Principal) {
	var marked []session.Principal
	return newFixture(func(dependencies *Dependencies) {
		dependencies.Sessions = fakeSessions{marked: &marked}
		if change != nil {
			change(dependencies)
		}
	}), &marked
}

func TestReauthenticateMarksTheSession(t *testing.T) {
	fixture, marked := reauthFixture(nil)
	principal := session.Principal{UserID: fixture.accounts.user.ID, SessionID: uuid.New()}

	require.NoError(t, fixture.service.Reauthenticate(t.Context(), principal, "right password", "198.51.100.1"))

	assert.Equal(t, []session.Principal{principal}, *marked)
}

func TestReauthenticateRejectsAWrongPassword(t *testing.T) {
	fixture, marked := reauthFixture(nil)
	principal := session.Principal{UserID: fixture.accounts.user.ID}

	err := fixture.service.Reauthenticate(t.Context(), principal, "wrong", "198.51.100.1")

	require.ErrorIs(t, err, ErrInvalidCredentials)
	assert.Empty(t, *marked)
	assert.Len(t, fixture.accounts.recorded, 1, "counts towards the login delays")
}

func TestReauthenticateRespectsLimitsAndLockouts(t *testing.T) {
	limited, _ := reauthFixture(func(dependencies *Dependencies) { dependencies.Limiter = fakeLimiter{deny: true} })
	require.ErrorIs(t, limited.service.Reauthenticate(t.Context(), session.Principal{}, "right password", "ip"), ErrRateLimited)

	locked, _ := reauthFixture(nil)
	until := now.Add(time.Minute)
	locked.accounts.user.LoginBlockedUntil = &until
	var delayed *DelayedError
	require.ErrorAs(t, locked.service.Reauthenticate(t.Context(), session.Principal{}, "right password", "ip"), &delayed)
	assert.Equal(t, time.Minute, delayed.RetryAfter)
}

func TestReauthenticateWithoutAPassword(t *testing.T) {
	fixture, _ := reauthFixture(nil)
	fixture.accounts.user.PasswordHash = nil

	require.ErrorIs(t, fixture.service.Reauthenticate(t.Context(), session.Principal{}, "anything", "ip"), ErrNoPassword)
}

func TestReauthenticateReportsLookupFailures(t *testing.T) {
	fixture, _ := reauthFixture(nil)
	fixture.accounts.lookupErr = errors.New("down")

	require.ErrorIs(t, fixture.service.Reauthenticate(t.Context(), session.Principal{}, "right password", "ip"), fixture.accounts.lookupErr)
}
