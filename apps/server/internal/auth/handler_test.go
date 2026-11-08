package auth

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// The handler is tested with real services over small fakes, since its job
// is mapping their results to HTTP responses.

type fakeAccounts struct {
	user      accountsstore.User
	lookupErr error
}

func (fake *fakeAccounts) GetUserByLogin(context.Context, string) (accountsstore.User, error) {
	if fake.lookupErr != nil {
		return accountsstore.User{}, fake.lookupErr
	}
	return fake.user, nil
}
func (fake *fakeAccounts) GetUserByID(context.Context, uuid.UUID) (accountsstore.User, error) {
	return fake.user, fake.lookupErr
}
func (*fakeAccounts) RecordFailedLogin(context.Context, accountsstore.RecordFailedLoginParams) error {
	return nil
}
func (*fakeAccounts) ResetFailedLogins(context.Context, uuid.UUID) error { return nil }
func (*fakeAccounts) UpdatePasswordHash(context.Context, accountsstore.UpdatePasswordHashParams) error {
	return nil
}

type plainHasher struct{}

func (plainHasher) Hash(_ context.Context, plain string) (string, error) { return plain, nil }
func (plainHasher) Verify(_ context.Context, plain string, encoded string) (bool, error) {
	return plain == encoded, nil
}
func (plainHasher) NeedsRehash(string) bool { return false }

type limiter struct{ deny bool }

func (fake limiter) Allow(string) bool { return !fake.deny }

type memorySessions struct {
	sessions map[uuid.UUID]sessionstore.Session
	fail     error
}

func (memory *memorySessions) InsertSession(_ context.Context, arg sessionstore.InsertSessionParams) error {
	memory.sessions[arg.ID] = sessionstore.Session(arg)
	return memory.fail
}
func (memory *memorySessions) GetSessionByTokenHash(context.Context, []byte) (sessionstore.Session, error) {
	return sessionstore.Session{}, pgx.ErrNoRows
}
func (*memorySessions) TouchSession(context.Context, sessionstore.TouchSessionParams) error {
	return nil
}
func (memory *memorySessions) ListSessionsForUser(_ context.Context, arg sessionstore.ListSessionsForUserParams) ([]sessionstore.Session, error) {
	var result []sessionstore.Session
	for _, found := range memory.sessions {
		if found.UserID == arg.UserID {
			result = append(result, found)
		}
	}
	return result, memory.fail
}
func (memory *memorySessions) MarkSessionAuthenticated(_ context.Context, arg sessionstore.MarkSessionAuthenticatedParams) (int64, error) {
	if memory.fail != nil {
		return 0, memory.fail
	}
	found, ok := memory.sessions[arg.ID]
	if !ok || found.UserID != arg.UserID {
		return 0, nil
	}
	found.AuthenticatedAt = arg.AuthenticatedAt
	memory.sessions[arg.ID] = found
	return 1, nil
}
func (memory *memorySessions) DeleteSession(_ context.Context, arg sessionstore.DeleteSessionParams) (int64, error) {
	if memory.fail != nil {
		return 0, memory.fail
	}
	if found, ok := memory.sessions[arg.ID]; ok && found.UserID == arg.UserID {
		delete(memory.sessions, arg.ID)
		return 1, nil
	}
	return 0, nil
}

var now = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

type fixture struct {
	accounts *fakeAccounts
	sessions *memorySessions
	handler  *Handler
	user     accountsstore.User
}

func newFixture(deny bool) fixture {
	hash := "right password"
	user := accountsstore.User{ID: uuid.New(), Username: "alice", Email: "alice@example.com", PasswordHash: &hash}
	accounts := &fakeAccounts{user: user}
	sessions := &memorySessions{sessions: map[uuid.UUID]sessionstore.Session{}}
	sessionService := session.NewService(sessions, clock.Fixed{Time: now})
	logins := login.NewService(login.Dependencies{
		Accounts: accounts, Hasher: plainHasher{}, Sessions: sessionService, Limiter: limiter{deny: deny}, Clock: clock.Fixed{Time: now},
		SecondFactors: noFactors{}, Challenges: nil,
	})
	handler := NewHandler(HandlerDependencies{Logins: logins, Sessions: sessionService, Clock: clock.Fixed{Time: now}})
	return fixture{accounts: accounts, sessions: sessions, handler: handler, user: user}
}

func loginRequest(password string, device string) api.LoginRequestObject {
	return api.LoginRequestObject{Body: &api.LoginJSONRequestBody{Login: "alice", Password: password, DeviceName: device}}
}

func TestLoginResponses(t *testing.T) {
	setup := newFixture(false)

	response, err := setup.handler.Login(t.Context(), loginRequest("right password", "  My   Laptop "))
	require.NoError(t, err)
	ok, isOK := response.(api.Login200JSONResponse)
	require.True(t, isOK)
	assert.Equal(t, "My Laptop", ok.Session.DeviceName)
	assert.True(t, ok.Session.Current)
	assert.Equal(t, "alice", ok.User.Username)

	response, err = setup.handler.Login(t.Context(), loginRequest("wrong", "x"))
	require.NoError(t, err)
	assert.IsType(t, api.Login401JSONResponse{}, response)
}

func TestLoginRateLimitAndDelayCarryRetryAfter(t *testing.T) {
	limited := newFixture(true)
	response, err := limited.handler.Login(t.Context(), loginRequest("right password", "x"))
	require.NoError(t, err)
	tooMany, ok := response.(api.Login429JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "rate_limited", tooMany.Body.Code)
	assert.Equal(t, 60, *tooMany.Headers.RetryAfter)

	delayed := newFixture(false)
	until := now.Add(1500 * time.Millisecond)
	delayed.accounts.user.LoginBlockedUntil = &until
	response, err = delayed.handler.Login(t.Context(), loginRequest("right password", "x"))
	require.NoError(t, err)
	tooMany, ok = response.(api.Login429JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "login_delayed", tooMany.Body.Code)
	assert.Equal(t, 2, *tooMany.Headers.RetryAfter, "rounded up to whole seconds")
}

func TestLoginPassesOnUnexpectedErrors(t *testing.T) {
	setup := newFixture(false)
	setup.accounts.lookupErr = errors.New("database down")

	_, err := setup.handler.Login(t.Context(), loginRequest("right password", "x"))

	assert.ErrorContains(t, err, "database down")
}

func TestSessionEndpoints(t *testing.T) {
	setup := newFixture(false)
	first, err := setup.handler.Login(t.Context(), loginRequest("right password", "Laptop"))
	require.NoError(t, err)
	current := first.(api.Login200JSONResponse).Session.Id
	ctx := session.WithPrincipal(t.Context(), session.Principal{UserID: setup.user.ID, SessionID: current})

	listed, err := setup.handler.ListSessions(ctx, api.ListSessionsRequestObject{})
	require.NoError(t, err)
	assert.Len(t, listed, 1)

	response, err := setup.handler.RevokeSession(ctx, api.RevokeSessionRequestObject{SessionId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.RevokeSession404JSONResponse{}, response)

	response, err = setup.handler.RevokeSession(ctx, api.RevokeSessionRequestObject{SessionId: current})
	require.NoError(t, err)
	assert.IsType(t, api.RevokeSession204Response{}, response)

	// Logging out when the session is already gone still succeeds.
	logout, err := setup.handler.Logout(ctx, api.LogoutRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.Logout204Response{}, logout)
}

func TestSessionEndpointsPassOnStoreErrors(t *testing.T) {
	setup := newFixture(false)
	setup.sessions.fail = errors.New("database down")
	ctx := session.WithPrincipal(t.Context(), session.Principal{UserID: setup.user.ID, SessionID: uuid.New()})

	_, err := setup.handler.ListSessions(ctx, api.ListSessionsRequestObject{})
	require.Error(t, err)
	_, err = setup.handler.RevokeSession(ctx, api.RevokeSessionRequestObject{SessionId: uuid.New()})
	require.Error(t, err)
	_, err = setup.handler.Logout(ctx, api.LogoutRequestObject{})
	assert.Error(t, err)
}

func TestProtectedHandlersRequireAPrincipal(t *testing.T) {
	setup := newFixture(false)

	_, err := setup.handler.Logout(t.Context(), api.LogoutRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = setup.handler.ListSessions(t.Context(), api.ListSessionsRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = setup.handler.RevokeSession(t.Context(), api.RevokeSessionRequestObject{})
	assert.ErrorIs(t, err, errNoPrincipal)
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// noFactors reports that no account has a second factor.
type noFactors struct{}

func (noFactors) HasSecondFactor(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (noFactors) Methods(context.Context, uuid.UUID) ([]string, error)     { return nil, nil }
func (noFactors) VerifyTOTP(context.Context, uuid.UUID, string) (bool, error) {
	return false, nil
}
func (noFactors) UseBackupCode(context.Context, uuid.UUID, string) (bool, error) {
	return false, nil
}
