package tokens

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens/store"
)

type fakeStore struct {
	tokens    map[string]store.InsertTokenParams
	insertErr error
	deleteErr error
	redeemErr error
}

func (fake *fakeStore) InsertToken(_ context.Context, arg store.InsertTokenParams) error {
	if fake.insertErr != nil {
		return fake.insertErr
	}
	fake.tokens[string(arg.TokenHash)] = arg
	return nil
}

func (fake *fakeStore) ConsumeToken(_ context.Context, arg store.ConsumeTokenParams) (uuid.UUID, error) {
	if fake.redeemErr != nil {
		return uuid.Nil, fake.redeemErr
	}
	token, ok := fake.tokens[string(arg.TokenHash)]
	if !ok || token.Purpose != arg.Purpose || !token.ExpiresAt.After(arg.Now) {
		return uuid.Nil, pgx.ErrNoRows
	}
	delete(fake.tokens, string(arg.TokenHash))
	return token.UserID, nil
}

func (fake *fakeStore) DeleteTokensForUser(_ context.Context, arg store.DeleteTokensForUserParams) error {
	for key, token := range fake.tokens {
		if token.UserID == arg.UserID && token.Purpose == arg.Purpose {
			delete(fake.tokens, key)
		}
	}
	return fake.deleteErr
}

type movableClock struct{ now time.Time }

func (clock *movableClock) Now() time.Time { return clock.now }

var start = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func newService() (*Service, *fakeStore, *movableClock) {
	fake := &fakeStore{tokens: map[string]store.InsertTokenParams{}}
	clock := &movableClock{now: start}
	return NewService(fake, clock), fake, clock
}

func TestIssueAndRedeemOnce(t *testing.T) {
	service, _, _ := newService()
	user := uuid.New()

	token, err := service.Issue(t.Context(), user, VerifyEmail)
	require.NoError(t, err)
	assert.Len(t, token, 43)

	redeemed, err := service.Redeem(t.Context(), VerifyEmail, token)
	require.NoError(t, err)
	assert.Equal(t, user, redeemed)
	_, err = service.Redeem(t.Context(), VerifyEmail, token)
	assert.ErrorIs(t, err, ErrInvalidToken, "single use")
}

func TestTokensAreBoundToTheirPurpose(t *testing.T) {
	service, _, _ := newService()
	token, err := service.Issue(t.Context(), uuid.New(), VerifyEmail)
	require.NoError(t, err)

	_, err = service.Redeem(t.Context(), ResetPassword, token)

	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokensExpire(t *testing.T) {
	service, _, clock := newService()
	reset, err := service.Issue(t.Context(), uuid.New(), ResetPassword)
	require.NoError(t, err)
	verify, err := service.Issue(t.Context(), uuid.New(), VerifyEmail)
	require.NoError(t, err)

	clock.now = start.Add(2 * time.Hour)

	_, err = service.Redeem(t.Context(), ResetPassword, reset)
	require.ErrorIs(t, err, ErrInvalidToken, "reset links last one hour")
	_, err = service.Redeem(t.Context(), VerifyEmail, verify)
	assert.NoError(t, err, "verification links last two days")
}

func TestNewTokenReplacesOlderOne(t *testing.T) {
	service, _, _ := newService()
	user := uuid.New()
	first, err := service.Issue(t.Context(), user, ResetPassword)
	require.NoError(t, err)
	_, err = service.Issue(t.Context(), user, ResetPassword)
	require.NoError(t, err)

	_, err = service.Redeem(t.Context(), ResetPassword, first)

	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestFailures(t *testing.T) {
	failure := errors.New("database down")

	service, fake, _ := newService()
	_, err := service.Issue(t.Context(), uuid.New(), Purpose("other"))
	require.ErrorContains(t, err, "unknown token purpose")

	fake.deleteErr = failure
	_, err = service.Issue(t.Context(), uuid.New(), VerifyEmail)
	require.ErrorIs(t, err, failure)

	fake.deleteErr, fake.insertErr = nil, failure
	_, err = service.Issue(t.Context(), uuid.New(), VerifyEmail)
	require.ErrorIs(t, err, failure)

	fake.redeemErr = failure
	_, err = service.Redeem(t.Context(), VerifyEmail, "x")
	assert.ErrorIs(t, err, failure)
}
