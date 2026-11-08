package auth

import (
	"context"
	"errors"
	"math"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passwordreset"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

const rateLimitRetrySeconds = 60

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

type emailVerifier interface {
	Verify(ctx context.Context, token string) error
}

type verificationSender interface {
	SendVerification(ctx context.Context, user accountsstore.User) error
}

// HandlerDependencies are the services the auth endpoints use.
type HandlerDependencies struct {
	Logins   *login.Service
	Sessions *session.Service
	Resets   *passwordreset.Service
	Verifier emailVerifier
	Emails   verificationSender
	Users    accounts.UserReader
	MFA      secondFactorManager
	Passkeys passkeyService
	// PasskeyLimiter limits passkey-login starts per client IP; each one
	// stores a challenge.
	PasskeyLimiter attemptLimiter
	// Clock decides whether a session logged in recently enough.
	Clock clock.Clock
}

type attemptLimiter interface {
	Allow(key string) bool
}

// Handler serves the auth endpoints.
type Handler struct {
	dependencies HandlerDependencies
}

// NewHandler returns a Handler.
func NewHandler(dependencies HandlerDependencies) *Handler {
	return &Handler{dependencies: dependencies}
}

// Login answers POST /api/v1/auth/login.
func (handler *Handler) Login(ctx context.Context, request api.LoginRequestObject) (api.LoginResponseObject, error) {
	result, err := handler.dependencies.Logins.Login(ctx, login.Request{
		Login:     request.Body.Login,
		Password:  request.Body.Password,
		Device:    session.Device{Name: apiresponse.DeviceName(request.Body.DeviceName)},
		ClientKey: clientip.FromContext(ctx).Address.String(),
	})
	var delayed *login.DelayedError
	switch {
	case errors.Is(err, login.ErrInvalidCredentials):
		return api.Login401JSONResponse(apiresponse.Error("invalid_credentials", err)), nil
	case errors.Is(err, login.ErrRateLimited):
		return tooManyAttempts("rate_limited", err, rateLimitRetrySeconds), nil
	case errors.As(err, &delayed):
		return tooManyAttempts("login_delayed", err, int(math.Ceil(delayed.RetryAfter.Seconds()))), nil
	case err != nil:
		return nil, err
	case result.MFAChallenge != "":
		methods := make([]api.MfaMethod, 0, len(result.MFAMethods))
		for _, method := range result.MFAMethods {
			methods = append(methods, api.MfaMethod(method))
		}
		return api.Login202JSONResponse{ChallengeToken: result.MFAChallenge, Methods: methods}, nil
	}
	return api.Login200JSONResponse{
		Token:   result.Token,
		Session: apiresponse.Session(result.Session, result.Session.ID),
		User:    accounts.CurrentUserResponse(result.User),
	}, nil
}

func tooManyAttempts(code string, err error, retryAfterSeconds int) api.Login429JSONResponse {
	return api.Login429JSONResponse{
		Body:    apiresponse.Error(code, err),
		Headers: api.Login429ResponseHeaders{RetryAfter: &retryAfterSeconds},
	}
}

// Logout answers POST /api/v1/auth/logout.
func (handler *Handler) Logout(ctx context.Context, _ api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.dependencies.Sessions.Revoke(ctx, principal.UserID, principal.SessionID)
	if err != nil && !errors.Is(err, session.ErrNotFound) {
		return nil, err
	}
	return api.Logout204Response{}, nil
}

// ListSessions answers GET /api/v1/auth/sessions.
func (handler *Handler) ListSessions(ctx context.Context, _ api.ListSessionsRequestObject) (api.ListSessionsResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	sessions, err := handler.dependencies.Sessions.List(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	response := make(api.ListSessions200JSONResponse, 0, len(sessions))
	for _, found := range sessions {
		response = append(response, apiresponse.Session(found, principal.SessionID))
	}
	return response, nil
}

// RevokeSession answers DELETE /api/v1/auth/sessions/{sessionId}.
func (handler *Handler) RevokeSession(ctx context.Context, request api.RevokeSessionRequestObject) (api.RevokeSessionResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.dependencies.Sessions.Revoke(ctx, principal.UserID, request.SessionId)
	if errors.Is(err, session.ErrNotFound) {
		return api.RevokeSession404JSONResponse(apiresponse.Error("session_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.RevokeSession204Response{}, nil
}
