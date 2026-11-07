package passwordreset

import (
	"bytes"
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
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

var (
	alice = accountsstore.User{ID: uuid.New(), Email: "alice@example.com"}
	now   = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
)

type fakeAccounts struct {
	lookupErr   error
	setErr      error
	verifyErr   error
	newPassword *string
	verified    bool
}

func (fake *fakeAccounts) GetUserByEmail(_ context.Context, email string) (accountsstore.User, error) {
	if fake.lookupErr != nil {
		return accountsstore.User{}, fake.lookupErr
	}
	if email != alice.Email {
		return accountsstore.User{}, pgx.ErrNoRows
	}
	return alice, nil
}

func (fake *fakeAccounts) SetPassword(_ context.Context, arg accountsstore.SetPasswordParams) error {
	fake.newPassword = arg.PasswordHash
	return fake.setErr
}

func (fake *fakeAccounts) MarkEmailVerified(context.Context, accountsstore.MarkEmailVerifiedParams) error {
	fake.verified = true
	return fake.verifyErr
}

type fakeSessions struct {
	revoked []uuid.UUID
	err     error
}

func (fake *fakeSessions) DeleteSessionsForUser(_ context.Context, userID uuid.UUID) error {
	fake.revoked = append(fake.revoked, userID)
	return fake.err
}

type fakeTokens struct{ err error }

func (fake fakeTokens) Redeem(_ context.Context, purpose tokens.Purpose, token string) (uuid.UUID, error) {
	if fake.err != nil {
		return uuid.Nil, fake.err
	}
	if purpose != tokens.ResetPassword || token != "good" {
		return uuid.Nil, tokens.ErrInvalidToken
	}
	return alice.ID, nil
}

type fakeMailer struct {
	sent []accountsstore.User
	err  error
}

func (fake *fakeMailer) SendPasswordReset(_ context.Context, user accountsstore.User) error {
	fake.sent = append(fake.sent, user)
	return fake.err
}

type fakeValidator struct{}

func (fakeValidator) CheckPassword(password string) error {
	if len(password) < 10 {
		return errors.New("too short")
	}
	return nil
}

type fakeHasher struct{ err error }

func (fake fakeHasher) Hash(_ context.Context, password string) (string, error) {
	return "hashed:" + password, fake.err
}

type fakeLimiter struct{ deny bool }

func (fake fakeLimiter) Allow(string) bool { return !fake.deny }

type fixture struct {
	accounts *fakeAccounts
	sessions *fakeSessions
	mailer   *fakeMailer
	logs     *bytes.Buffer
	service  *Service
}

func newFixture(change func(*Dependencies)) fixture {
	setup := fixture{accounts: &fakeAccounts{}, sessions: &fakeSessions{}, mailer: &fakeMailer{}, logs: &bytes.Buffer{}}
	dependencies := Dependencies{
		Accounts: setup.accounts, Sessions: setup.sessions, Tokens: fakeTokens{}, Mailer: setup.mailer,
		Validator: fakeValidator{}, Hasher: fakeHasher{}, Limiter: fakeLimiter{}, Clock: clock.Fixed{Time: now},
		Logger: slog.New(slog.NewTextHandler(setup.logs, nil)),
	}
	if change != nil {
		change(&dependencies)
	}
	setup.service = NewService(dependencies)
	return setup
}

func TestRequestSendsOnlyToKnownAddresses(t *testing.T) {
	setup := newFixture(nil)

	require.NoError(t, setup.service.Request(t.Context(), "alice@example.com", "client"))
	require.NoError(t, setup.service.Request(t.Context(), "nobody@example.com", "client"))

	assert.Len(t, setup.mailer.sent, 1, "unknown addresses get no email, and no different answer")
}

func TestRequestLogsButHidesProblems(t *testing.T) {
	setup := newFixture(nil)
	setup.mailer.err = errors.New("SMTP down")
	require.NoError(t, setup.service.Request(t.Context(), "alice@example.com", "client"))
	assert.Contains(t, setup.logs.String(), "SMTP down")

	broken := newFixture(nil)
	broken.accounts.lookupErr = errors.New("database down")
	require.NoError(t, broken.service.Request(t.Context(), "alice@example.com", "client"))
	assert.Contains(t, broken.logs.String(), "database down")
}

func TestRequestIsRateLimited(t *testing.T) {
	setup := newFixture(func(d *Dependencies) { d.Limiter = fakeLimiter{deny: true} })

	assert.ErrorIs(t, setup.service.Request(t.Context(), "alice@example.com", "client"), ErrRateLimited)
}

func TestCompleteSetsPasswordVerifiesEmailAndLogsOutEverywhere(t *testing.T) {
	setup := newFixture(nil)

	require.NoError(t, setup.service.Complete(t.Context(), "good", "a new long password"))

	require.NotNil(t, setup.accounts.newPassword)
	assert.Equal(t, "hashed:a new long password", *setup.accounts.newPassword)
	assert.True(t, setup.accounts.verified)
	assert.Equal(t, []uuid.UUID{alice.ID}, setup.sessions.revoked)
}

func TestCompleteRejectsWeakPasswordBeforeUsingTheToken(t *testing.T) {
	setup := newFixture(nil)

	err := setup.service.Complete(t.Context(), "good", "short")

	var problems *validation.Error
	require.ErrorAs(t, err, &problems)
	assert.Equal(t, "password", problems.Fields[0].Field)
	assert.Nil(t, setup.accounts.newPassword)
}

func TestCompleteFailures(t *testing.T) {
	failure := errors.New("failure")
	tests := map[string]struct {
		change  func(*Dependencies)
		prepare func(*fixture)
		token   string
		want    error
	}{
		"bad token":     {token: "bad", want: tokens.ErrInvalidToken},
		"token store":   {change: func(d *Dependencies) { d.Tokens = fakeTokens{err: failure} }, token: "good", want: failure},
		"hashing":       {change: func(d *Dependencies) { d.Hasher = fakeHasher{err: failure} }, token: "good", want: failure},
		"save password": {prepare: func(f *fixture) { f.accounts.setErr = failure }, token: "good", want: failure},
		"mark verified": {prepare: func(f *fixture) { f.accounts.verifyErr = failure }, token: "good", want: failure},
		"revoke":        {prepare: func(f *fixture) { f.sessions.err = failure }, token: "good", want: failure},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			setup := newFixture(test.change)
			if test.prepare != nil {
				test.prepare(&setup)
			}

			assert.ErrorIs(t, setup.service.Complete(t.Context(), test.token, "a new long password"), test.want)
		})
	}
}
