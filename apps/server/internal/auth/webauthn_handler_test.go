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
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// fakePasskeys returns err from every method and otherwise succeeds.
type fakePasskeys struct {
	err    error
	userID uuid.UUID
}

var started = passkeys.Started{CeremonyID: uuid.New(), Options: []byte(`{"publicKey":{"challenge":"abc"}}`)}

func (fake fakePasskeys) BeginRegistration(context.Context, uuid.UUID, string, string) (passkeys.Started, error) {
	return started, fake.err
}
func (fake fakePasskeys) FinishRegistration(context.Context, passkeys.Answer) error { return fake.err }
func (fake fakePasskeys) BeginSecondFactor(context.Context, uuid.UUID) (passkeys.Started, error) {
	return started, fake.err
}
func (fake fakePasskeys) FinishSecondFactor(context.Context, uuid.UUID, []byte) (uuid.UUID, error) {
	return fake.userID, fake.err
}
func (fake fakePasskeys) BeginPasskeyLogin(context.Context) (passkeys.Started, error) {
	return started, fake.err
}
func (fake fakePasskeys) FinishPasskeyLogin(context.Context, uuid.UUID, []byte) (uuid.UUID, error) {
	return fake.userID, fake.err
}
func (fake fakePasskeys) List(context.Context, uuid.UUID) ([]passkeys.Credential, error) {
	return []passkeys.Credential{{ID: uuid.New(), Kind: passkeys.KindPasskey, Name: "Phone", CreatedAt: time.Now()}}, fake.err
}
func (fake fakePasskeys) Remove(context.Context, uuid.UUID, uuid.UUID) error { return fake.err }

// webauthnHandler builds a handler with real login and session services
// whose user always has a second factor.
func webauthnHandler(keys fakePasskeys, deny bool) (*Handler, *memoryChallenges) {
	hash := "right password"
	accounts := &fakeAccounts{user: accountsstore.User{ID: uuid.New(), Username: "alice", PasswordHash: &hash}}
	challenges := &memoryChallenges{issued: map[string]uuid.UUID{}}
	logins := login.NewService(login.Dependencies{
		Accounts: accounts, Hasher: plainHasher{}, Sessions: session.NewService(&memorySessions{sessions: map[uuid.UUID]sessionstore.Session{}}, clock.Fixed{Time: now}),
		Limiter: limiter{deny: deny}, Clock: clock.Fixed{Time: now}, SecondFactors: alwaysFactors{}, Challenges: challenges,
	})
	return NewHandler(HandlerDependencies{Logins: logins, Passkeys: keys, PasskeyLimiter: limiter{deny: deny}, MFA: &fakeManager{firstCodes: []string{"AAAAA-BBBBB"}}}), challenges
}

func TestSecurityKeyLogin(t *testing.T) {
	handler, challenges := webauthnHandler(fakePasskeys{userID: uuid.New()}, false)
	challenges.issued["challenge"] = uuid.New()

	begun, err := handler.BeginSecurityKeyLogin(t.Context(), api.BeginSecurityKeyLoginRequestObject{Body: &api.ChallengeRequest{ChallengeToken: "challenge"}})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"challenge": "abc"}, begun.(api.BeginSecurityKeyLogin200JSONResponse).Options["publicKey"])

	finished, err := handler.FinishSecurityKeyLogin(t.Context(), api.FinishSecurityKeyLoginRequestObject{Body: &api.WebAuthnLoginAnswer{CeremonyId: uuid.New(), DeviceName: "Laptop"}})
	require.NoError(t, err)
	assert.Equal(t, "nxs_", finished.(api.FinishSecurityKeyLogin200JSONResponse).Token[:4])

	again, err := handler.BeginSecurityKeyLogin(t.Context(), api.BeginSecurityKeyLoginRequestObject{Body: &api.ChallengeRequest{ChallengeToken: "challenge"}})
	require.NoError(t, err)
	assert.IsType(t, api.BeginSecurityKeyLogin401JSONResponse{}, again, "the challenge was used up")
}

func TestSecurityKeyLoginErrors(t *testing.T) {
	begin := func(keys fakePasskeys, deny bool) (any, error) {
		handler, challenges := webauthnHandler(keys, deny)
		challenges.issued["challenge"] = uuid.New()
		return handler.BeginSecurityKeyLogin(t.Context(), api.BeginSecurityKeyLoginRequestObject{Body: &api.ChallengeRequest{ChallengeToken: "challenge"}})
	}
	response, err := begin(fakePasskeys{}, true)
	require.NoError(t, err)
	assert.IsType(t, api.BeginSecurityKeyLogin429JSONResponse{}, response)
	response, err = begin(fakePasskeys{err: passkeys.ErrNoDomain}, false)
	require.NoError(t, err)
	assert.IsType(t, api.BeginSecurityKeyLogin409JSONResponse{}, response)
	response, err = begin(fakePasskeys{err: passkeys.ErrNoCredentials}, false)
	require.NoError(t, err)
	assert.IsType(t, api.BeginSecurityKeyLogin401JSONResponse{}, response)
	_, err = begin(fakePasskeys{err: errors.New("down")}, false)
	require.Error(t, err)

	broken, challenges := webauthnHandler(fakePasskeys{}, false)
	challenges.err = errors.New("database down")
	_, err = broken.BeginSecurityKeyLogin(t.Context(), api.BeginSecurityKeyLoginRequestObject{Body: &api.ChallengeRequest{ChallengeToken: "x"}})
	require.Error(t, err)

	finish := func(keys fakePasskeys) (any, error) {
		handler, _ := webauthnHandler(keys, false)
		return handler.FinishSecurityKeyLogin(t.Context(), api.FinishSecurityKeyLoginRequestObject{Body: &api.WebAuthnLoginAnswer{}})
	}
	for _, rejected := range []error{passkeys.ErrCeremonyNotFound, passkeys.ErrVerificationFails} {
		response, err = finish(fakePasskeys{err: rejected})
		require.NoError(t, err)
		assert.IsType(t, api.FinishSecurityKeyLogin401JSONResponse{}, response)
	}
	_, err = finish(fakePasskeys{err: errors.New("down")})
	require.Error(t, err)
}

func TestPasskeyLogin(t *testing.T) {
	handler, _ := webauthnHandler(fakePasskeys{userID: uuid.New()}, false)

	begun, err := handler.BeginPasskeyLogin(t.Context(), api.BeginPasskeyLoginRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.BeginPasskeyLogin200JSONResponse{}, begun)
	finished, err := handler.FinishPasskeyLogin(t.Context(), api.FinishPasskeyLoginRequestObject{Body: &api.WebAuthnLoginAnswer{DeviceName: "Phone"}})
	require.NoError(t, err)
	assert.IsType(t, api.FinishPasskeyLogin200JSONResponse{}, finished)

	limited, _ := webauthnHandler(fakePasskeys{}, true)
	response, err := limited.BeginPasskeyLogin(t.Context(), api.BeginPasskeyLoginRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.BeginPasskeyLogin429JSONResponse{}, response)

	noDomain, _ := webauthnHandler(fakePasskeys{err: passkeys.ErrNoDomain}, false)
	response, err = noDomain.BeginPasskeyLogin(t.Context(), api.BeginPasskeyLoginRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.BeginPasskeyLogin409JSONResponse{}, response)

	rejected, _ := webauthnHandler(fakePasskeys{err: passkeys.ErrVerificationFails}, false)
	rejectedResponse, err := rejected.FinishPasskeyLogin(t.Context(), api.FinishPasskeyLoginRequestObject{Body: &api.WebAuthnLoginAnswer{}})
	require.NoError(t, err)
	assert.IsType(t, api.FinishPasskeyLogin401JSONResponse{}, rejectedResponse)

	failing, _ := webauthnHandler(fakePasskeys{err: errors.New("down")}, false)
	_, err = failing.BeginPasskeyLogin(t.Context(), api.BeginPasskeyLoginRequestObject{})
	require.Error(t, err)
	_, err = failing.FinishPasskeyLogin(t.Context(), api.FinishPasskeyLoginRequestObject{Body: &api.WebAuthnLoginAnswer{}})
	assert.Error(t, err)
}

func TestSessionStartFailureAfterWebAuthn(t *testing.T) {
	handler, _ := webauthnHandler(fakePasskeys{userID: uuid.New()}, false)
	handler.dependencies.Logins = login.NewService(login.Dependencies{Accounts: &fakeAccounts{lookupErr: errors.New("down")}})

	_, err := handler.FinishPasskeyLogin(t.Context(), api.FinishPasskeyLoginRequestObject{Body: &api.WebAuthnLoginAnswer{}})
	require.Error(t, err)
	_, err = handler.FinishSecurityKeyLogin(t.Context(), api.FinishSecurityKeyLoginRequestObject{Body: &api.WebAuthnLoginAnswer{}})
	assert.Error(t, err)
}
