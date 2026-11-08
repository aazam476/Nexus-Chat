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
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/mfa"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

type fakeManager struct {
	confirmCodes []string
	err          error
	confirmErr   error
	removeErr    error
	remainingErr error
	firstCodes   []string
	firstErr     error
}

func (fake *fakeManager) EnrollTOTP(_ context.Context, _ uuid.UUID, accountName string, name string) (mfa.Enrollment, error) {
	return mfa.Enrollment{ID: uuid.New(), Secret: "SECRET", URL: "otpauth://totp/x:" + accountName + "?" + name}, fake.err
}
func (fake *fakeManager) ConfirmTOTP(context.Context, uuid.UUID, uuid.UUID, string) ([]string, error) {
	return fake.confirmCodes, fake.confirmErr
}
func (fake *fakeManager) ListTOTP(context.Context, uuid.UUID) ([]mfa.Authenticator, error) {
	return []mfa.Authenticator{{ID: uuid.New(), Name: "Phone", CreatedAt: time.Now()}}, fake.err
}
func (fake *fakeManager) RemoveTOTP(context.Context, uuid.UUID, uuid.UUID) error {
	return fake.removeErr
}
func (fake *fakeManager) GenerateBackupCodes(context.Context, uuid.UUID) ([]string, error) {
	return []string{"AAAAA-BBBBB"}, fake.err
}
func (fake *fakeManager) BackupCodesIfFirstFactor(context.Context, uuid.UUID) ([]string, error) {
	return fake.firstCodes, fake.firstErr
}
func (fake *fakeManager) RemainingBackupCodes(context.Context, uuid.UUID) (int64, error) {
	return 9, fake.remainingErr
}

func signedIn(t *testing.T) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New(), SessionID: uuid.New(), AuthenticatedAt: now})
}

func TestSecondFactorEndpoints(t *testing.T) {
	manager := &fakeManager{confirmCodes: []string{"AAAAA-BBBBB"}}
	handler := NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: manager, Users: fakeUsers{user: accountsstore.User{Username: "alice"}}})
	ctx := signedIn(t)

	listed, err := handler.GetSecondFactors(ctx, api.GetSecondFactorsRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, int64(9), listed.(api.GetSecondFactors200JSONResponse).BackupCodesRemaining)

	enrolled, err := handler.EnrollTotp(ctx, api.EnrollTotpRequestObject{Body: &api.EnrollTotpRequest{Name: " Phone "}})
	require.NoError(t, err)
	assert.Contains(t, enrolled.(api.EnrollTotp201JSONResponse).OtpauthUrl, "alice?Phone")

	confirmed, err := handler.ConfirmTotp(ctx, api.ConfirmTotpRequestObject{EnrollmentId: uuid.New(), Body: &api.CodeRequest{Code: "123456"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"AAAAA-BBBBB"}, *confirmed.(api.ConfirmTotp200JSONResponse).BackupCodes)

	removed, err := handler.RemoveTotp(ctx, api.RemoveTotpRequestObject{AuthenticatorId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.RemoveTotp204Response{}, removed)

	regenerated, err := handler.RegenerateBackupCodes(ctx, api.RegenerateBackupCodesRequestObject{})
	require.NoError(t, err)
	assert.Len(t, regenerated.(api.RegenerateBackupCodes200JSONResponse).Codes, 1)
}

func TestSecondFactorEndpointErrors(t *testing.T) {
	ctx := signedIn(t)
	confirm := func(err error) any {
		handler := NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: &fakeManager{confirmErr: err}})
		response, handlerErr := handler.ConfirmTotp(ctx, api.ConfirmTotpRequestObject{Body: &api.CodeRequest{}})
		if handlerErr != nil {
			return handlerErr
		}
		return response
	}
	assert.IsType(t, api.ConfirmTotp400JSONResponse{}, confirm(mfa.ErrWrongCode))
	assert.IsType(t, api.ConfirmTotp400JSONResponse{}, confirm(mfa.ErrEnrollmentNotFound))
	assert.Implements(t, (*error)(nil), confirm(errors.New("down")))

	noCodes, err := NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: &fakeManager{}}).ConfirmTotp(ctx, api.ConfirmTotpRequestObject{Body: &api.CodeRequest{}})
	require.NoError(t, err)
	assert.Nil(t, noCodes.(api.ConfirmTotp200JSONResponse).BackupCodes)

	missing, err := NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: &fakeManager{removeErr: mfa.ErrNotFound}}).RemoveTotp(ctx, api.RemoveTotpRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.RemoveTotp404JSONResponse{}, missing)
	_, err = NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: &fakeManager{removeErr: errors.New("down")}}).RemoveTotp(ctx, api.RemoveTotpRequestObject{})
	require.Error(t, err)

	failing := NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: &fakeManager{err: errors.New("down")}, Users: fakeUsers{}})
	_, err = failing.GetSecondFactors(ctx, api.GetSecondFactorsRequestObject{})
	require.Error(t, err)
	_, err = failing.EnrollTotp(ctx, api.EnrollTotpRequestObject{Body: &api.EnrollTotpRequest{}})
	require.Error(t, err)
	_, err = failing.RegenerateBackupCodes(ctx, api.RegenerateBackupCodesRequestObject{})
	require.Error(t, err)
	_, err = NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: &fakeManager{remainingErr: errors.New("down")}}).GetSecondFactors(ctx, api.GetSecondFactorsRequestObject{})
	require.Error(t, err)
	_, err = NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: &fakeManager{}, Users: fakeUsers{err: errors.New("down")}}).EnrollTotp(ctx, api.EnrollTotpRequestObject{Body: &api.EnrollTotpRequest{}})
	assert.Error(t, err)
}

func TestSecondFactorEndpointsNeedAPrincipal(t *testing.T) {
	handler := NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: &fakeManager{}})

	_, err := handler.GetSecondFactors(t.Context(), api.GetSecondFactorsRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.EnrollTotp(t.Context(), api.EnrollTotpRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.ConfirmTotp(t.Context(), api.ConfirmTotpRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.RemoveTotp(t.Context(), api.RemoveTotpRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.RegenerateBackupCodes(t.Context(), api.RegenerateBackupCodesRequestObject{})
	assert.ErrorIs(t, err, errNoPrincipal)
}

// alwaysFactors reports a second factor for every account.
type alwaysFactors struct{ accept bool }

func (alwaysFactors) HasSecondFactor(context.Context, uuid.UUID) (bool, error) { return true, nil }
func (alwaysFactors) Methods(context.Context, uuid.UUID) ([]string, error) {
	return []string{"totp", "backup_code"}, nil
}
func (factors alwaysFactors) VerifyTOTP(context.Context, uuid.UUID, string) (bool, error) {
	return factors.accept, nil
}
func (alwaysFactors) UseBackupCode(context.Context, uuid.UUID, string) (bool, error) {
	return false, nil
}

type memoryChallenges struct {
	issued map[string]uuid.UUID
	err    error
}

func (memory *memoryChallenges) Issue(_ context.Context, userID uuid.UUID, _ tokens.Purpose) (string, error) {
	memory.issued["challenge"] = userID
	return "challenge", nil
}
func (memory *memoryChallenges) Redeem(_ context.Context, _ tokens.Purpose, token string) (uuid.UUID, error) {
	if memory.err != nil {
		return uuid.Nil, memory.err
	}
	userID, ok := memory.issued[token]
	if !ok {
		return uuid.Nil, tokens.ErrInvalidToken
	}
	delete(memory.issued, token)
	return userID, nil
}

func mfaHandler(accept bool, deny bool) *Handler {
	hash := "right password"
	accounts := &fakeAccounts{user: accountsstore.User{ID: uuid.New(), Username: "alice", PasswordHash: &hash}}
	sessions := &memorySessions{sessions: map[uuid.UUID]sessionstore.Session{}}
	logins := login.NewService(login.Dependencies{
		Accounts: accounts, Hasher: plainHasher{}, Sessions: session.NewService(sessions, clock.Fixed{Time: now}),
		Limiter: limiter{deny: deny}, Clock: clock.Fixed{Time: now},
		SecondFactors: alwaysFactors{accept: accept}, Challenges: &memoryChallenges{issued: map[string]uuid.UUID{}},
	})
	return NewHandler(HandlerDependencies{Logins: logins})
}

func TestLoginWithSecondFactor(t *testing.T) {
	handler := mfaHandler(true, false)

	first, err := handler.Login(t.Context(), loginRequest("right password", "Laptop"))
	require.NoError(t, err)
	challenge, ok := first.(api.Login202JSONResponse)
	require.True(t, ok)
	assert.Equal(t, []api.MfaMethod{api.MfaMethodTotp, api.MfaMethodBackupCode}, challenge.Methods)

	body := &api.MfaLoginRequest{ChallengeToken: challenge.ChallengeToken, Method: api.MfaMethodTotp, Code: "123456", DeviceName: "Laptop"}
	second, err := handler.CompleteLoginMfa(t.Context(), api.CompleteLoginMfaRequestObject{Body: body})
	require.NoError(t, err)
	assert.IsType(t, api.CompleteLoginMfa200JSONResponse{}, second)

	again, err := handler.CompleteLoginMfa(t.Context(), api.CompleteLoginMfaRequestObject{Body: body})
	require.NoError(t, err)
	assert.IsType(t, api.CompleteLoginMfa401JSONResponse{}, again, "the challenge was used up")
}

func TestCompleteLoginMfaRateLimit(t *testing.T) {
	body := &api.MfaLoginRequest{ChallengeToken: "x", Method: api.MfaMethodTotp}

	response, err := mfaHandler(true, true).CompleteLoginMfa(t.Context(), api.CompleteLoginMfaRequestObject{Body: body})

	require.NoError(t, err)
	assert.IsType(t, api.CompleteLoginMfa429JSONResponse{}, response)
}

func TestCompleteLoginMfaPassesOnUnexpectedErrors(t *testing.T) {
	logins := login.NewService(login.Dependencies{
		Limiter: limiter{}, Clock: clock.Fixed{Time: now},
		Challenges: &memoryChallenges{err: errors.New("database down")},
	})
	body := &api.MfaLoginRequest{ChallengeToken: "x", Method: api.MfaMethodTotp}

	_, err := NewHandler(HandlerDependencies{Logins: logins}).CompleteLoginMfa(t.Context(), api.CompleteLoginMfaRequestObject{Body: body})

	assert.ErrorContains(t, err, "database down")
}
