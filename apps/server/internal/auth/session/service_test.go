package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
)

// fakeStore keeps sessions in memory and can be told to fail.
type fakeStore struct {
	sessions map[string]store.Session // keyed by token hash
	touches  int
	fail     error
}

func newFakeStore() *fakeStore { return &fakeStore{sessions: map[string]store.Session{}} }

func (fake *fakeStore) InsertSession(_ context.Context, arg store.InsertSessionParams) error {
	if fake.fail != nil {
		return fake.fail
	}
	fake.sessions[string(arg.TokenHash)] = store.Session(arg)
	return nil
}

func (fake *fakeStore) GetSessionByTokenHash(_ context.Context, hash []byte) (store.Session, error) {
	if fake.fail != nil {
		return store.Session{}, fake.fail
	}
	session, ok := fake.sessions[string(hash)]
	if !ok {
		return store.Session{}, pgx.ErrNoRows
	}
	return session, nil
}

func (fake *fakeStore) TouchSession(_ context.Context, arg store.TouchSessionParams) error {
	fake.touches++
	for hash, session := range fake.sessions {
		if session.ID == arg.ID {
			session.LastSeenAt, session.ExpiresAt = arg.LastSeenAt, arg.ExpiresAt
			fake.sessions[hash] = session
		}
	}
	return nil
}

func (fake *fakeStore) ListSessionsForUser(_ context.Context, arg store.ListSessionsForUserParams) ([]store.Session, error) {
	if fake.fail != nil {
		return nil, fake.fail
	}
	var result []store.Session
	for _, session := range fake.sessions {
		if session.UserID == arg.UserID && session.ExpiresAt.After(arg.ExpiresAt) {
			result = append(result, session)
		}
	}
	return result, nil
}

func (fake *fakeStore) DeleteSession(_ context.Context, arg store.DeleteSessionParams) (int64, error) {
	if fake.fail != nil {
		return 0, fake.fail
	}
	for hash, session := range fake.sessions {
		if session.ID == arg.ID && session.UserID == arg.UserID {
			delete(fake.sessions, hash)
			return 1, nil
		}
	}
	return 0, nil
}

type stepClock struct{ now time.Time }

func (clock *stepClock) Now() time.Time { return clock.now }

var (
	start  = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	alice  = uuid.MustParse("01900000-0000-7000-8000-000000000001")
	device = Device{Name: "Laptop", UserAgent: "test"}
)

func TestCreateThenAuthenticate(t *testing.T) {
	sessions := newFakeStore()
	service := NewService(sessions, &stepClock{now: start})

	token, created, err := service.Create(t.Context(), alice, device)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(token, "nxs_"))
	assert.Len(t, token, len("nxs_")+43) // 32 bytes in unpadded base64url
	assert.NotContains(t, string(created.TokenHash), token, "only the hash is stored")
	assert.Equal(t, start.Add(Lifetime), created.ExpiresAt)
	found, err := service.Authenticate(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, alice, found.UserID)
}

func TestTokensAreUnique(t *testing.T) {
	service := NewService(newFakeStore(), &stepClock{now: start})

	first, _, err := service.Create(t.Context(), alice, device)
	require.NoError(t, err)
	second, _, err := service.Create(t.Context(), alice, device)
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestAuthenticateRejectsBadTokens(t *testing.T) {
	service := NewService(newFakeStore(), &stepClock{now: start})

	for _, token := range []string{"", "Bearer nxs_x", "nxs_unknown", "abc"} {
		_, err := service.Authenticate(t.Context(), token)
		assert.ErrorIs(t, err, ErrInvalidSession, token) //nolint:testifylint // each token is checked independently
	}
}

func TestSessionsExpireAfterInactivity(t *testing.T) {
	clock := &stepClock{now: start}
	service := NewService(newFakeStore(), clock)
	token, _, err := service.Create(t.Context(), alice, device)
	require.NoError(t, err)

	clock.now = start.Add(Lifetime)
	_, err = service.Authenticate(t.Context(), token)

	assert.ErrorIs(t, err, ErrInvalidSession)
}

func TestActivityExtendsSessionButIsWrittenRarely(t *testing.T) {
	sessions := newFakeStore()
	clock := &stepClock{now: start}
	service := NewService(sessions, clock)
	token, _, err := service.Create(t.Context(), alice, device)
	require.NoError(t, err)

	clock.now = start.Add(time.Minute)
	_, err = service.Authenticate(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, 0, sessions.touches, "used again within 5 minutes: no write")

	clock.now = start.Add(80 * 24 * time.Hour)
	extended, err := service.Authenticate(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, 1, sessions.touches)
	assert.Equal(t, clock.now.Add(Lifetime), extended.ExpiresAt)

	clock.now = start.Add(100 * 24 * time.Hour) // past the original expiry
	_, err = service.Authenticate(t.Context(), token)
	assert.NoError(t, err, "sliding expiry kept the session alive")
}

func TestListAndRevoke(t *testing.T) {
	service := NewService(newFakeStore(), &stepClock{now: start})
	token, first, err := service.Create(t.Context(), alice, device)
	require.NoError(t, err)
	_, _, err = service.Create(t.Context(), alice, Device{Name: "Phone"})
	require.NoError(t, err)
	_, other, err := service.Create(t.Context(), uuid.New(), device)
	require.NoError(t, err)

	listed, err := service.List(t.Context(), alice)
	require.NoError(t, err)
	assert.Len(t, listed, 2)

	require.ErrorIs(t, service.Revoke(t.Context(), alice, other.ID), ErrNotFound, "cannot revoke another user's session")
	require.NoError(t, service.Revoke(t.Context(), alice, first.ID))
	_, err = service.Authenticate(t.Context(), token)
	require.ErrorIs(t, err, ErrInvalidSession)
	require.ErrorIs(t, service.Revoke(t.Context(), alice, first.ID), ErrNotFound)
}

func TestStoreErrorsArePassedOn(t *testing.T) {
	failure := errors.New("database down")
	sessions := newFakeStore()
	sessions.fail = failure
	service := NewService(sessions, &stepClock{now: start})

	_, _, err := service.Create(t.Context(), alice, device)
	require.ErrorIs(t, err, failure)
	_, err = service.Authenticate(t.Context(), "nxs_anything")
	require.ErrorIs(t, err, failure)
	_, err = service.List(t.Context(), alice)
	require.ErrorIs(t, err, failure)
	assert.ErrorIs(t, service.Revoke(t.Context(), alice, uuid.New()), failure)
}

// failingTouchStore fails only when recording activity.
type failingTouchStore struct{ *fakeStore }

func (failingTouchStore) TouchSession(context.Context, store.TouchSessionParams) error {
	return errors.New("touch failed")
}

func TestTouchErrorsArePassedOn(t *testing.T) {
	clock := &stepClock{now: start}
	service := NewService(failingTouchStore{newFakeStore()}, clock)
	token, _, err := service.Create(t.Context(), alice, device)
	require.NoError(t, err)
	clock.now = start.Add(time.Hour)

	_, err = service.Authenticate(t.Context(), token)

	assert.ErrorContains(t, err, "touch failed")
}
