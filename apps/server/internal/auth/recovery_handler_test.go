package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passwordreset"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

type fakeVerifier struct{ err error }

func (fake fakeVerifier) Verify(context.Context, string) error { return fake.err }

type fakeSender struct{ err error }

func (fake fakeSender) SendVerification(context.Context, accountsstore.User) error { return fake.err }

type fakeUsers struct {
	user accountsstore.User
	err  error
}

func (fake fakeUsers) GetUserByID(context.Context, uuid.UUID) (accountsstore.User, error) {
	return fake.user, fake.err
}

func TestVerifyEmailResponses(t *testing.T) {
	ok, err := NewHandler(HandlerDependencies{Verifier: fakeVerifier{}}).VerifyEmail(t.Context(), api.VerifyEmailRequestObject{Body: &api.TokenRequest{Token: "x"}})
	require.NoError(t, err)
	assert.IsType(t, api.VerifyEmail204Response{}, ok)

	bad, err := NewHandler(HandlerDependencies{Verifier: fakeVerifier{err: tokens.ErrInvalidToken}}).VerifyEmail(t.Context(), api.VerifyEmailRequestObject{Body: &api.TokenRequest{Token: "x"}})
	require.NoError(t, err)
	assert.IsType(t, api.VerifyEmail400JSONResponse{}, bad)

	_, err = NewHandler(HandlerDependencies{Verifier: fakeVerifier{err: errors.New("down")}}).VerifyEmail(t.Context(), api.VerifyEmailRequestObject{Body: &api.TokenRequest{Token: "x"}})
	assert.Error(t, err)
}

func TestResendVerificationResponses(t *testing.T) {
	ctx := session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New()})
	verifiedAt := time.Now()
	tests := map[string]struct {
		dependencies HandlerDependencies
		want         any
	}{
		"sent":             {HandlerDependencies{Users: fakeUsers{}, Emails: fakeSender{}}, api.ResendVerificationEmail202Response{}},
		"already verified": {HandlerDependencies{Users: fakeUsers{user: accountsstore.User{EmailVerifiedAt: &verifiedAt}}}, api.ResendVerificationEmail409JSONResponse{}},
		"no email":         {HandlerDependencies{Users: fakeUsers{}, Emails: fakeSender{err: errors.New("no SMTP")}}, api.ResendVerificationEmail503JSONResponse{}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			response, err := NewHandler(test.dependencies).ResendVerificationEmail(ctx, api.ResendVerificationEmailRequestObject{})

			require.NoError(t, err)
			assert.IsType(t, test.want, response)
		})
	}

	_, err := NewHandler(HandlerDependencies{Users: fakeUsers{err: errors.New("down")}}).ResendVerificationEmail(ctx, api.ResendVerificationEmailRequestObject{})
	require.Error(t, err)
	_, err = NewHandler(HandlerDependencies{}).ResendVerificationEmail(t.Context(), api.ResendVerificationEmailRequestObject{})
	assert.ErrorIs(t, err, errNoPrincipal)
}

// The reset service is used for real, over the same small fakes as its own
// tests would use.
type resetAccounts struct{}

func (resetAccounts) GetUserByEmail(context.Context, string) (accountsstore.User, error) {
	return accountsstore.User{}, errors.New("not used")
}
func (resetAccounts) SetPassword(context.Context, accountsstore.SetPasswordParams) error { return nil }
func (resetAccounts) MarkEmailVerified(context.Context, accountsstore.MarkEmailVerifiedParams) error {
	return nil
}

type resetSessions struct{ err error }

func (fake resetSessions) DeleteSessionsForUser(context.Context, uuid.UUID) error { return fake.err }

type resetTokens struct{}

func (resetTokens) Redeem(_ context.Context, _ tokens.Purpose, token string) (uuid.UUID, error) {
	if token != "good" {
		return uuid.Nil, tokens.ErrInvalidToken
	}
	return uuid.New(), nil
}

type resetMailer struct{}

func (resetMailer) SendPasswordReset(context.Context, accountsstore.User) error { return nil }

type lengthValidator struct{}

func (lengthValidator) CheckPassword(password string) error {
	if len(password) < 10 {
		return errors.New("too short")
	}
	return nil
}

func newResetHandler(deny bool, sessions resetSessions) *Handler {
	resets := passwordreset.NewService(passwordreset.Dependencies{
		Accounts: resetAccounts{}, Sessions: sessions, Tokens: resetTokens{}, Mailer: resetMailer{},
		Validator: lengthValidator{}, Hasher: plainHasher{}, Limiter: limiter{deny: deny}, Clock: clock.Fixed{},
		Logger: discard(),
	})
	return NewHandler(HandlerDependencies{Resets: resets})
}

func TestPasswordResetResponses(t *testing.T) {
	request := api.RequestPasswordResetRequestObject{Body: &api.PasswordResetRequest{Email: "a@example.com"}}
	accepted, err := newResetHandler(false, resetSessions{}).RequestPasswordReset(t.Context(), request)
	require.NoError(t, err)
	assert.IsType(t, api.RequestPasswordReset202Response{}, accepted)
	limited, err := newResetHandler(true, resetSessions{}).RequestPasswordReset(t.Context(), request)
	require.NoError(t, err)
	assert.IsType(t, api.RequestPasswordReset429JSONResponse{}, limited)

	complete := func(token string, password string, sessions resetSessions) (api.CompletePasswordResetResponseObject, error) {
		body := &api.CompletePasswordResetRequest{Token: token, Password: password}
		return newResetHandler(false, sessions).CompletePasswordReset(t.Context(), api.CompletePasswordResetRequestObject{Body: body})
	}
	done, err := complete("good", "a long password", resetSessions{})
	require.NoError(t, err)
	assert.IsType(t, api.CompletePasswordReset204Response{}, done)
	invalid, err := complete("bad", "a long password", resetSessions{})
	require.NoError(t, err)
	assert.IsType(t, api.CompletePasswordReset400JSONResponse{}, invalid)
	weak, err := complete("good", "short", resetSessions{})
	require.NoError(t, err)
	assert.IsType(t, api.CompletePasswordReset422JSONResponse{}, weak)
	_, err = complete("good", "a long password", resetSessions{err: errors.New("down")})
	assert.Error(t, err)
}
