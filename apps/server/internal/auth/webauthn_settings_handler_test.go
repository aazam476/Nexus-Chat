package auth

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

func settingsHandler(keys fakePasskeys, manager *fakeManager) *Handler {
	return NewHandler(HandlerDependencies{Clock: clock.Fixed{Time: now}, Passkeys: keys, MFA: manager})
}

func TestWebAuthnSettings(t *testing.T) {
	ctx := signedIn(t)
	handler := settingsHandler(fakePasskeys{}, &fakeManager{firstCodes: []string{"AAAAA-BBBBB"}})

	listed, err := handler.ListWebAuthnCredentials(ctx, api.ListWebAuthnCredentialsRequestObject{})
	require.NoError(t, err)
	assert.Len(t, listed, 1)

	begun, err := handler.BeginWebAuthnRegistration(ctx, api.BeginWebAuthnRegistrationRequestObject{Body: &api.WebAuthnRegistrationRequest{Kind: api.WebAuthnKindPasskey, Name: "Phone"}})
	require.NoError(t, err)
	assert.IsType(t, api.BeginWebAuthnRegistration200JSONResponse{}, begun)

	finished, err := handler.FinishWebAuthnRegistration(ctx, api.FinishWebAuthnRegistrationRequestObject{Body: &api.WebAuthnRegistrationAnswer{Kind: api.WebAuthnKindPasskey}})
	require.NoError(t, err)
	assert.Equal(t, []string{"AAAAA-BBBBB"}, *finished.(api.FinishWebAuthnRegistration201JSONResponse).BackupCodes)

	noCodes, err := settingsHandler(fakePasskeys{}, &fakeManager{}).FinishWebAuthnRegistration(ctx, api.FinishWebAuthnRegistrationRequestObject{Body: &api.WebAuthnRegistrationAnswer{}})
	require.NoError(t, err)
	assert.Nil(t, noCodes.(api.FinishWebAuthnRegistration201JSONResponse).BackupCodes)

	removed, err := handler.RemoveWebAuthnCredential(ctx, api.RemoveWebAuthnCredentialRequestObject{CredentialId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.RemoveWebAuthnCredential204Response{}, removed)
}

func TestWebAuthnSettingsErrors(t *testing.T) {
	ctx := signedIn(t)
	failure := errors.New("down")

	begin, err := settingsHandler(fakePasskeys{err: passkeys.ErrNoDomain}, &fakeManager{}).BeginWebAuthnRegistration(ctx, api.BeginWebAuthnRegistrationRequestObject{Body: &api.WebAuthnRegistrationRequest{}})
	require.NoError(t, err)
	assert.IsType(t, api.BeginWebAuthnRegistration409JSONResponse{}, begin)

	for _, rejected := range []error{passkeys.ErrCeremonyNotFound, passkeys.ErrVerificationFails} {
		response, err := settingsHandler(fakePasskeys{err: rejected}, &fakeManager{}).FinishWebAuthnRegistration(ctx, api.FinishWebAuthnRegistrationRequestObject{Body: &api.WebAuthnRegistrationAnswer{}})
		require.NoError(t, err)
		assert.IsType(t, api.FinishWebAuthnRegistration400JSONResponse{}, response)
	}

	missing, err := settingsHandler(fakePasskeys{err: passkeys.ErrNotFound}, &fakeManager{}).RemoveWebAuthnCredential(ctx, api.RemoveWebAuthnCredentialRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.RemoveWebAuthnCredential404JSONResponse{}, missing)

	failing := settingsHandler(fakePasskeys{err: failure}, &fakeManager{})
	_, err = failing.ListWebAuthnCredentials(ctx, api.ListWebAuthnCredentialsRequestObject{})
	require.Error(t, err)
	_, err = failing.BeginWebAuthnRegistration(ctx, api.BeginWebAuthnRegistrationRequestObject{Body: &api.WebAuthnRegistrationRequest{}})
	require.Error(t, err)
	_, err = failing.FinishWebAuthnRegistration(ctx, api.FinishWebAuthnRegistrationRequestObject{Body: &api.WebAuthnRegistrationAnswer{}})
	require.Error(t, err)
	_, err = failing.RemoveWebAuthnCredential(ctx, api.RemoveWebAuthnCredentialRequestObject{})
	require.Error(t, err)
	_, err = settingsHandler(fakePasskeys{}, &fakeManager{firstErr: failure}).FinishWebAuthnRegistration(ctx, api.FinishWebAuthnRegistrationRequestObject{Body: &api.WebAuthnRegistrationAnswer{}})
	require.Error(t, err)

	unauthenticated := settingsHandler(fakePasskeys{}, &fakeManager{})
	_, err = unauthenticated.ListWebAuthnCredentials(t.Context(), api.ListWebAuthnCredentialsRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = unauthenticated.BeginWebAuthnRegistration(t.Context(), api.BeginWebAuthnRegistrationRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = unauthenticated.FinishWebAuthnRegistration(t.Context(), api.FinishWebAuthnRegistrationRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = unauthenticated.RemoveWebAuthnCredential(t.Context(), api.RemoveWebAuthnCredentialRequestObject{})
	assert.ErrorIs(t, err, errNoPrincipal)
}
