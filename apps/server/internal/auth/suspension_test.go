package auth

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// suspendedHandler signs in a suspended account, with a second factor when mfa is set.
func suspendedHandler(mfa bool) (*Handler, *memoryChallenges, *fakeAccounts) {
	hash := "right password"
	suspended := now
	accounts := &fakeAccounts{user: accountsstore.User{ID: uuid.New(), Username: "alice", PasswordHash: &hash, SuspendedAt: &suspended}}
	challenges := &memoryChallenges{issued: map[string]uuid.UUID{}}
	logins := login.NewService(login.Dependencies{
		Accounts: accounts, Hasher: plainHasher{}, Sessions: session.NewService(&memorySessions{sessions: map[uuid.UUID]sessionstore.Session{}}, clock.Fixed{Time: now}),
		Limiter: limiter{}, Clock: clock.Fixed{Time: now}, SecondFactors: noFactors{}, Challenges: challenges,
	})
	if mfa {
		logins = login.NewService(login.Dependencies{
			Accounts: accounts, Hasher: plainHasher{}, Sessions: session.NewService(&memorySessions{sessions: map[uuid.UUID]sessionstore.Session{}}, clock.Fixed{Time: now}),
			Limiter: limiter{}, Clock: clock.Fixed{Time: now}, SecondFactors: alwaysFactors{accept: true}, Challenges: challenges,
		})
	}
	handler := NewHandler(HandlerDependencies{Logins: logins, Passkeys: fakePasskeys{userID: accounts.user.ID}, PasskeyLimiter: limiter{}, MFA: &fakeManager{}})
	return handler, challenges, accounts
}

func TestSuspendedAccountsAreTurnedAway(t *testing.T) {
	handler, challenges, _ := suspendedHandler(false)
	response, err := handler.Login(t.Context(), loginRequest("right password", "Laptop"))
	require.NoError(t, err)
	refused, ok := response.(api.Login403JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "account_suspended", refused.Code)

	passkey, err := handler.FinishPasskeyLogin(t.Context(), api.FinishPasskeyLoginRequestObject{Body: &api.WebAuthnLoginAnswer{DeviceName: "Phone"}})
	require.NoError(t, err)
	assert.IsType(t, api.FinishPasskeyLogin403JSONResponse{}, passkey)

	challenges.issued["challenge"] = uuid.New()
	_, err = handler.BeginSecurityKeyLogin(t.Context(), api.BeginSecurityKeyLoginRequestObject{Body: &api.ChallengeRequest{ChallengeToken: "challenge"}})
	require.NoError(t, err)
	key, err := handler.FinishSecurityKeyLogin(t.Context(), api.FinishSecurityKeyLoginRequestObject{Body: &api.WebAuthnLoginAnswer{CeremonyId: uuid.New(), DeviceName: "Laptop"}})
	require.NoError(t, err)
	assert.IsType(t, api.FinishSecurityKeyLogin403JSONResponse{}, key)
}

func TestAnAccountSuspendedMidSignInIsTurnedAway(t *testing.T) {
	handler, _, accounts := suspendedHandler(true)
	accounts.user.SuspendedAt = nil
	first, err := handler.Login(t.Context(), loginRequest("right password", "Laptop"))
	require.NoError(t, err)
	challenge, ok := first.(api.Login202JSONResponse)
	require.True(t, ok)

	suspended := now
	accounts.user.SuspendedAt = &suspended
	body := &api.MfaLoginRequest{ChallengeToken: challenge.ChallengeToken, Method: api.MfaMethodTotp, Code: "123456", DeviceName: "Laptop"}
	second, err := handler.CompleteLoginMfa(t.Context(), api.CompleteLoginMfaRequestObject{Body: body})
	require.NoError(t, err)
	assert.IsType(t, api.CompleteLoginMfa403JSONResponse{}, second)
}
