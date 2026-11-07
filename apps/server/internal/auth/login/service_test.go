package login

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/password"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

var now = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

type fakeAccounts struct {
	user      accountsstore.User
	missing   bool
	lookupErr error
	writeErr  error
	recorded  []accountsstore.RecordFailedLoginParams
	resets    int
	newHash   *string
}

func (fake *fakeAccounts) GetUserByLogin(_ context.Context, login string) (accountsstore.User, error) {
	if fake.lookupErr != nil {
		return accountsstore.User{}, fake.lookupErr
	}
	if fake.missing || (login != fake.user.Username && login != fake.user.Email) {
		return accountsstore.User{}, pgx.ErrNoRows
	}
	return fake.user, nil
}

func (fake *fakeAccounts) RecordFailedLogin(_ context.Context, arg accountsstore.RecordFailedLoginParams) error {
	fake.recorded = append(fake.recorded, arg)
	return fake.writeErr
}

func (fake *fakeAccounts) ResetFailedLogins(context.Context, uuid.UUID) error {
	fake.resets++
	return fake.writeErr
}

func (fake *fakeAccounts) UpdatePasswordHash(_ context.Context, arg accountsstore.UpdatePasswordHashParams) error {
	fake.newHash = arg.PasswordHash
	return fake.writeErr
}

type fakeHasher struct {
	verifyCalls []string // hashes verified
	verifyErr   error
	hashErr     error
	outdated    bool
}

func (fake *fakeHasher) Hash(_ context.Context, plain string) (string, error) {
	return "new:" + plain, fake.hashErr
}

func (fake *fakeHasher) Verify(_ context.Context, plain string, encoded string) (bool, error) {
	fake.verifyCalls = append(fake.verifyCalls, encoded)
	return encoded == "hash:"+plain, fake.verifyErr
}

func (fake *fakeHasher) NeedsRehash(string) bool { return fake.outdated }

type fakeSessions struct{ err error }

func (fake fakeSessions) Create(_ context.Context, userID uuid.UUID, device session.Device) (string, sessionstore.Session, error) {
	return "nxs_token", sessionstore.Session{UserID: userID, DeviceName: device.Name}, fake.err
}

type fakeLimiter struct{ deny bool }

func (fake fakeLimiter) Allow(string) bool { return !fake.deny }

func passwordHash(plain string) *string {
	hash := "hash:" + plain
	return &hash
}

type fixture struct {
	accounts *fakeAccounts
	hasher   *fakeHasher
	service  *Service
}

func newFixture(change func(*Dependencies)) fixture {
	accounts := &fakeAccounts{user: accountsstore.User{
		ID: uuid.New(), Username: "alice", Email: "alice@example.com", PasswordHash: passwordHash("right password"),
	}}
	hasher := &fakeHasher{}
	dependencies := Dependencies{Accounts: accounts, Hasher: hasher, Sessions: fakeSessions{}, Limiter: fakeLimiter{}, Clock: clock.Fixed{Time: now}}
	if change != nil {
		change(&dependencies)
	}
	return fixture{accounts: accounts, hasher: hasher, service: NewService(dependencies)}
}

func attempt(login string, plain string) Request {
	return Request{Login: login, Password: plain, Device: session.Device{Name: "Laptop"}, ClientKey: "198.51.100.1"}
}

func TestLoginWithUsernameOrEmail(t *testing.T) {
	for _, login := range []string{"alice", "alice@example.com"} {
		t.Run(login, func(t *testing.T) {
			setup := newFixture(nil)

			result, err := setup.service.Login(t.Context(), attempt(login, "right password"))

			require.NoError(t, err)
			assert.Equal(t, "nxs_token", result.Token)
			assert.Equal(t, setup.accounts.user.ID, result.Session.UserID)
			assert.Equal(t, "Laptop", result.Session.DeviceName)
			assert.Zero(t, setup.accounts.resets, "nothing to reset")
		})
	}
}

func TestWrongPasswordIsRecorded(t *testing.T) {
	setup := newFixture(nil)

	_, err := setup.service.Login(t.Context(), attempt("alice", "wrong"))

	require.ErrorIs(t, err, ErrInvalidCredentials)
	require.Len(t, setup.accounts.recorded, 1)
	assert.Nil(t, setup.accounts.recorded[0].LoginBlockedUntil, "first failure is free")
}

func TestUnknownAccountsCostTheSameTime(t *testing.T) {
	for name, change := range map[string]func(*fakeAccounts){
		"no such user":         func(a *fakeAccounts) { a.missing = true },
		"passwordless account": func(a *fakeAccounts) { a.user.PasswordHash = nil },
	} {
		t.Run(name, func(t *testing.T) {
			setup := newFixture(nil)
			change(setup.accounts)

			_, err := setup.service.Login(t.Context(), attempt("alice", "whatever"))

			require.ErrorIs(t, err, ErrInvalidCredentials)
			assert.Equal(t, []string{dummyHash}, setup.hasher.verifyCalls, "a hash is still verified")
		})
	}
}

func TestDummyHashIsAValidHash(t *testing.T) {
	hasher, err := password.NewHasher(password.DefaultParams(), 1)
	require.NoError(t, err)

	match, err := hasher.Verify(t.Context(), "anything", dummyHash)

	require.NoError(t, err, "the dummy hash must parse, or timing would differ")
	assert.False(t, match)
	assert.False(t, hasher.NeedsRehash(dummyHash), "the dummy hash must use the production cost")
}

func TestLockedAccountIsRefusedBeforeCheckingThePassword(t *testing.T) {
	setup := newFixture(nil)
	until := now.Add(90 * time.Second)
	setup.accounts.user.LoginBlockedUntil = &until

	_, err := setup.service.Login(t.Context(), attempt("alice", "right password"))

	var delayed *DelayedError
	require.ErrorAs(t, err, &delayed)
	assert.Equal(t, 90*time.Second, delayed.RetryAfter)
	assert.Equal(t, "too many wrong passwords; try again in 1m30s", err.Error())
	assert.Empty(t, setup.hasher.verifyCalls)
}

func TestExpiredLockAllowsLogin(t *testing.T) {
	setup := newFixture(nil)
	past := now.Add(-time.Second)
	setup.accounts.user.LoginBlockedUntil = &past
	setup.accounts.user.FailedLoginCount = 7

	_, err := setup.service.Login(t.Context(), attempt("alice", "right password"))

	require.NoError(t, err)
	assert.Equal(t, 1, setup.accounts.resets, "a success clears the failure count")
}

func TestOutdatedHashIsUpgraded(t *testing.T) {
	setup := newFixture(nil)
	setup.hasher.outdated = true

	_, err := setup.service.Login(t.Context(), attempt("alice", "right password"))

	require.NoError(t, err)
	require.NotNil(t, setup.accounts.newHash)
	assert.Equal(t, "new:right password", *setup.accounts.newHash)
}

func TestRateLimitedBeforeAnyLookup(t *testing.T) {
	setup := newFixture(func(d *Dependencies) { d.Limiter = fakeLimiter{deny: true} })

	_, err := setup.service.Login(t.Context(), attempt("alice", "right password"))

	require.ErrorIs(t, err, ErrRateLimited)
	assert.Empty(t, setup.hasher.verifyCalls)
}

func TestBlockAfterFailure(t *testing.T) {
	tests := []struct {
		failures int32
		want     time.Duration // 0 means not blocked
	}{
		{1, 0}, {4, 0}, {5, time.Second}, {6, 2 * time.Second}, {7, 4 * time.Second},
		{14, 512 * time.Second}, {15, 15 * time.Minute}, {40, 15 * time.Minute},
	}
	for _, test := range tests {
		until := blockAfterFailure(test.failures, now)
		if test.want == 0 {
			assert.Nil(t, until, "failure %d", test.failures)
			continue
		}
		require.NotNil(t, until, "failure %d", test.failures)
		assert.Equal(t, test.want, until.Sub(now), "failure %d", test.failures)
	}
}

func TestFailuresArePassedOn(t *testing.T) {
	failure := errors.New("database down")
	tests := map[string]struct {
		prepare  func(*fixture)
		password string
	}{
		"lookup":      {prepare: func(f *fixture) { f.accounts.lookupErr = failure }, password: "right password"},
		"verify":      {prepare: func(f *fixture) { f.hasher.verifyErr = failure }, password: "right password"},
		"record":      {prepare: func(f *fixture) { f.accounts.writeErr = failure }, password: "wrong"},
		"reset":       {prepare: func(f *fixture) { f.accounts.user.FailedLoginCount = 1; f.accounts.writeErr = failure }, password: "right password"},
		"rehash":      {prepare: func(f *fixture) { f.hasher.outdated = true; f.hasher.hashErr = failure }, password: "right password"},
		"save rehash": {prepare: func(f *fixture) { f.hasher.outdated = true; f.accounts.writeErr = failure }, password: "right password"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			setup := newFixture(nil)
			test.prepare(&setup)

			_, err := setup.service.Login(t.Context(), attempt("alice", test.password))

			assert.ErrorIs(t, err, failure)
		})
	}
}

func TestSessionFailureIsPassedOn(t *testing.T) {
	failure := errors.New("cannot save session")
	setup := newFixture(func(d *Dependencies) { d.Sessions = fakeSessions{err: failure} })

	_, err := setup.service.Login(t.Context(), attempt("alice", "right password"))

	assert.ErrorIs(t, err, failure)
}
