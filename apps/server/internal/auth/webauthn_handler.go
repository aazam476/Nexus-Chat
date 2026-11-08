package auth

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
)

var errPasskeyRateLimited = errors.New("too many passkey login attempts from this address; wait a minute")

// passkeyService is satisfied by *passkeys.Service.
type passkeyService interface {
	BeginRegistration(ctx context.Context, userID uuid.UUID, kind string, name string) (passkeys.Started, error)
	FinishRegistration(ctx context.Context, answer passkeys.Answer) error
	BeginSecondFactor(ctx context.Context, userID uuid.UUID) (passkeys.Started, error)
	FinishSecondFactor(ctx context.Context, ceremonyID uuid.UUID, response []byte) (uuid.UUID, error)
	BeginPasskeyLogin(ctx context.Context) (passkeys.Started, error)
	FinishPasskeyLogin(ctx context.Context, ceremonyID uuid.UUID, response []byte) (uuid.UUID, error)
	List(ctx context.Context, userID uuid.UUID) ([]passkeys.Credential, error)
	Remove(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

// ceremony converts a started ceremony to the API shape. The options are
// JSON the library produced, so decoding them cannot fail.
func ceremony(started passkeys.Started) api.WebAuthnCeremony {
	var options map[string]any
	_ = json.Unmarshal(started.Options, &options)
	return api.WebAuthnCeremony{CeremonyId: started.CeremonyID, Options: options}
}

// encodeCredential turns the decoded request object back into the JSON the
// library parses. It came from JSON, so encoding cannot fail.
func encodeCredential(credential map[string]any) []byte {
	encoded, _ := json.Marshal(credential)
	return encoded
}

func webAuthnUnavailable(err error) api.Error {
	return apiresponse.Error("webauthn_unavailable", err)
}

// BeginSecurityKeyLogin answers POST /api/v1/auth/login/mfa/security-key/begin.
func (handler *Handler) BeginSecurityKeyLogin(ctx context.Context, request api.BeginSecurityKeyLoginRequestObject) (api.BeginSecurityKeyLoginResponseObject, error) {
	userID, err := handler.dependencies.Logins.RedeemChallenge(ctx, request.Body.ChallengeToken, clientip.FromContext(ctx).Address.String())
	switch {
	case errors.Is(err, login.ErrInvalidSecondFactor):
		return api.BeginSecurityKeyLogin401JSONResponse(apiresponse.Error("invalid_second_factor", err)), nil
	case errors.Is(err, login.ErrRateLimited):
		return api.BeginSecurityKeyLogin429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case err != nil:
		return nil, err
	}
	started, err := handler.dependencies.Passkeys.BeginSecondFactor(ctx, userID)
	if errors.Is(err, passkeys.ErrNoDomain) {
		return api.BeginSecurityKeyLogin409JSONResponse{WebAuthnUnavailableJSONResponse: api.WebAuthnUnavailableJSONResponse(webAuthnUnavailable(err))}, nil
	}
	if errors.Is(err, passkeys.ErrNoCredentials) {
		return api.BeginSecurityKeyLogin401JSONResponse(apiresponse.Error("invalid_second_factor", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.BeginSecurityKeyLogin200JSONResponse(ceremony(started)), nil
}

// FinishSecurityKeyLogin answers POST /api/v1/auth/login/mfa/security-key/finish.
func (handler *Handler) FinishSecurityKeyLogin(ctx context.Context, request api.FinishSecurityKeyLoginRequestObject) (api.FinishSecurityKeyLoginResponseObject, error) {
	userID, err := handler.dependencies.Passkeys.FinishSecondFactor(ctx, request.Body.CeremonyId, encodeCredential(request.Body.Credential))
	if errors.Is(err, passkeys.ErrCeremonyNotFound) || errors.Is(err, passkeys.ErrVerificationFails) {
		return api.FinishSecurityKeyLogin401JSONResponse(apiresponse.Error("invalid_second_factor", err)), nil
	}
	if err != nil {
		return nil, err
	}
	result, err := handler.dependencies.Logins.StartSession(ctx, userID, session.Device{Name: apiresponse.DeviceName(request.Body.DeviceName)})
	if err != nil {
		return nil, err
	}
	return api.FinishSecurityKeyLogin200JSONResponse(loginResult(result)), nil
}

// BeginPasskeyLogin answers POST /api/v1/auth/passkey/begin.
func (handler *Handler) BeginPasskeyLogin(ctx context.Context, _ api.BeginPasskeyLoginRequestObject) (api.BeginPasskeyLoginResponseObject, error) {
	if !handler.dependencies.PasskeyLimiter.Allow(clientip.FromContext(ctx).Address.String()) {
		return api.BeginPasskeyLogin429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", errPasskeyRateLimited))}, nil
	}
	started, err := handler.dependencies.Passkeys.BeginPasskeyLogin(ctx)
	if errors.Is(err, passkeys.ErrNoDomain) {
		return api.BeginPasskeyLogin409JSONResponse{WebAuthnUnavailableJSONResponse: api.WebAuthnUnavailableJSONResponse(webAuthnUnavailable(err))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.BeginPasskeyLogin200JSONResponse(ceremony(started)), nil
}

// FinishPasskeyLogin answers POST /api/v1/auth/passkey/finish.
func (handler *Handler) FinishPasskeyLogin(ctx context.Context, request api.FinishPasskeyLoginRequestObject) (api.FinishPasskeyLoginResponseObject, error) {
	userID, err := handler.dependencies.Passkeys.FinishPasskeyLogin(ctx, request.Body.CeremonyId, encodeCredential(request.Body.Credential))
	if errors.Is(err, passkeys.ErrCeremonyNotFound) || errors.Is(err, passkeys.ErrVerificationFails) {
		return api.FinishPasskeyLogin401JSONResponse(apiresponse.Error("invalid_passkey", err)), nil
	}
	if err != nil {
		return nil, err
	}
	result, err := handler.dependencies.Logins.StartSession(ctx, userID, session.Device{Name: apiresponse.DeviceName(request.Body.DeviceName)})
	if err != nil {
		return nil, err
	}
	return api.FinishPasskeyLogin200JSONResponse(loginResult(result)), nil
}

func loginResult(result login.Result) api.LoginResult {
	return api.LoginResult{
		Token: result.Token, Session: apiresponse.Session(result.Session, result.Session.ID), User: accounts.CurrentUserResponse(result.User),
	}
}
