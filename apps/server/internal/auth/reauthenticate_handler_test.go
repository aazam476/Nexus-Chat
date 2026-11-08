package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

func reauthenticate(t *testing.T, setup fixture, ctx context.Context, password string) api.ReauthenticateResponseObject {
	t.Helper()
	response, err := setup.handler.Reauthenticate(ctx, api.ReauthenticateRequestObject{Body: &api.ReauthenticateRequest{Password: password}})
	require.NoError(t, err)
	return response
}

func TestReauthenticateResponses(t *testing.T) {
	setup := newFixture(false)
	_, created, err := setup.handler.dependencies.Sessions.Create(t.Context(), setup.user.ID, session.Device{Name: "Laptop"})
	require.NoError(t, err)
	ctx := session.WithPrincipal(t.Context(), session.Principal{UserID: setup.user.ID, SessionID: created.ID})

	assert.IsType(t, api.Reauthenticate204Response{}, reauthenticate(t, setup, ctx, "right password"))
	assert.Equal(t, now, setup.sessions.sessions[created.ID].AuthenticatedAt)

	wrong := reauthenticate(t, setup, ctx, "wrong").(api.Reauthenticate403JSONResponse)
	assert.Equal(t, "invalid_credentials", wrong.Code)

	revoked := session.WithPrincipal(t.Context(), session.Principal{UserID: setup.user.ID, SessionID: uuid.New()})
	assert.IsType(t, api.Reauthenticate401JSONResponse{}, reauthenticate(t, setup, revoked, "right password"))

	setup.accounts.user.PasswordHash = nil
	assert.Equal(t, "no_password", reauthenticate(t, setup, ctx, "x").(api.Reauthenticate403JSONResponse).Code)
}

func TestReauthenticateLimits(t *testing.T) {
	limited := newFixture(true)
	ctx := session.WithPrincipal(t.Context(), session.Principal{UserID: limited.user.ID})
	tooMany := reauthenticate(t, limited, ctx, "right password").(api.Reauthenticate429JSONResponse)
	assert.Equal(t, "rate_limited", tooMany.Body.Code)
	assert.Equal(t, 60, *tooMany.Headers.RetryAfter)

	locked := newFixture(false)
	until := now.Add(90 * time.Second)
	locked.accounts.user.LoginBlockedUntil = &until
	delayed := reauthenticate(t, locked, ctx, "right password").(api.Reauthenticate429JSONResponse)
	assert.Equal(t, "login_delayed", delayed.Body.Code)
	assert.Equal(t, 90, *delayed.Headers.RetryAfter)

	broken := newFixture(false)
	broken.accounts.lookupErr = errors.New("down")
	_, err := broken.handler.Reauthenticate(ctx, api.ReauthenticateRequestObject{Body: &api.ReauthenticateRequest{}})
	require.Error(t, err)
	_, err = broken.handler.Reauthenticate(t.Context(), api.ReauthenticateRequestObject{Body: &api.ReauthenticateRequest{}})
	require.ErrorIs(t, err, errNoPrincipal)
}

func TestChangesToHowTheAccountLogsInNeedARecentLogin(t *testing.T) {
	stale := session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New(), AuthenticatedAt: now.Add(-session.RecentAuthentication - time.Second)})
	handler := NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, MFA: &fakeManager{}, Passkeys: fakePasskeys{}})

	removeTotp, err := handler.RemoveTotp(stale, api.RemoveTotpRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, "reauthentication_required", removeTotp.(api.RemoveTotp403JSONResponse).Code)
	codes, err := handler.RegenerateBackupCodes(stale, api.RegenerateBackupCodesRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.RegenerateBackupCodes403JSONResponse{}, codes)
	begin, err := handler.BeginWebAuthnRegistration(stale, api.BeginWebAuthnRegistrationRequestObject{Body: &api.WebAuthnRegistrationRequest{}})
	require.NoError(t, err)
	assert.IsType(t, api.BeginWebAuthnRegistration403JSONResponse{}, begin)
	remove, err := handler.RemoveWebAuthnCredential(stale, api.RemoveWebAuthnCredentialRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.RemoveWebAuthnCredential403JSONResponse{}, remove)

	_, err = handler.RemoveTotp(t.Context(), api.RemoveTotpRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.RegenerateBackupCodes(t.Context(), api.RegenerateBackupCodesRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.RemoveWebAuthnCredential(t.Context(), api.RemoveWebAuthnCredentialRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
}
