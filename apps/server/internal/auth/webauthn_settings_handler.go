package auth

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// ListWebAuthnCredentials answers GET /api/v1/auth/webauthn.
func (handler *Handler) ListWebAuthnCredentials(ctx context.Context, _ api.ListWebAuthnCredentialsRequestObject) (api.ListWebAuthnCredentialsResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	credentials, err := handler.dependencies.Passkeys.List(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	response := make(api.ListWebAuthnCredentials200JSONResponse, 0, len(credentials))
	for _, credential := range credentials {
		response = append(response, api.WebAuthnCredential{
			Id: credential.ID, Kind: api.WebAuthnKind(credential.Kind), Name: credential.Name,
			CreatedAt: credential.CreatedAt, LastUsedAt: credential.LastUsedAt,
		})
	}
	return response, nil
}

// BeginWebAuthnRegistration answers POST /api/v1/auth/webauthn/register/begin.
func (handler *Handler) BeginWebAuthnRegistration(ctx context.Context, request api.BeginWebAuthnRegistrationRequestObject) (api.BeginWebAuthnRegistrationResponseObject, error) {
	principal, recent, err := handler.recentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if !recent {
		return api.BeginWebAuthnRegistration403JSONResponse{ReauthenticationRequiredJSONResponse: reauthenticationRequired()}, nil
	}
	started, err := handler.dependencies.Passkeys.BeginRegistration(ctx, principal.UserID, string(request.Body.Kind), apiresponse.DeviceName(request.Body.Name))
	if errors.Is(err, passkeys.ErrNoDomain) {
		return api.BeginWebAuthnRegistration409JSONResponse{WebAuthnUnavailableJSONResponse: api.WebAuthnUnavailableJSONResponse(webAuthnUnavailable(err))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.BeginWebAuthnRegistration200JSONResponse(ceremony(started)), nil
}

// FinishWebAuthnRegistration answers POST /api/v1/auth/webauthn/register/finish.
func (handler *Handler) FinishWebAuthnRegistration(ctx context.Context, request api.FinishWebAuthnRegistrationRequestObject) (api.FinishWebAuthnRegistrationResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.dependencies.Passkeys.FinishRegistration(ctx, passkeys.Answer{
		UserID: principal.UserID, CeremonyID: request.Body.CeremonyId, Kind: string(request.Body.Kind),
		Response: encodeCredential(request.Body.Credential),
	})
	switch {
	case errors.Is(err, passkeys.ErrCeremonyNotFound):
		return api.FinishWebAuthnRegistration400JSONResponse(apiresponse.Error("ceremony_not_found", err)), nil
	case errors.Is(err, passkeys.ErrVerificationFails):
		return api.FinishWebAuthnRegistration400JSONResponse(apiresponse.Error("invalid_credential", err)), nil
	case err != nil:
		return nil, err
	}
	codes, err := handler.dependencies.MFA.BackupCodesIfFirstFactor(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	response := api.FinishWebAuthnRegistration201JSONResponse{}
	if codes != nil {
		response.BackupCodes = &codes
	}
	return response, nil
}

// RemoveWebAuthnCredential answers DELETE /api/v1/auth/webauthn/{credentialId}.
func (handler *Handler) RemoveWebAuthnCredential(ctx context.Context, request api.RemoveWebAuthnCredentialRequestObject) (api.RemoveWebAuthnCredentialResponseObject, error) {
	principal, recent, err := handler.recentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if !recent {
		return api.RemoveWebAuthnCredential403JSONResponse{ReauthenticationRequiredJSONResponse: reauthenticationRequired()}, nil
	}
	err = handler.dependencies.Passkeys.Remove(ctx, principal.UserID, request.CredentialId)
	if errors.Is(err, passkeys.ErrNotFound) {
		return api.RemoveWebAuthnCredential404JSONResponse(apiresponse.Error("credential_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.RemoveWebAuthnCredential204Response{}, nil
}
