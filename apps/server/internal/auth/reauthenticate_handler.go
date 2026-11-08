package auth

import (
	"context"
	"errors"
	"math"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
)

// Reauthenticate answers POST /api/v1/auth/reauthenticate.
func (handler *Handler) Reauthenticate(ctx context.Context, request api.ReauthenticateRequestObject) (api.ReauthenticateResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.dependencies.Logins.Reauthenticate(ctx, principal, request.Body.Password, clientip.FromContext(ctx).Address.String())
	var delayed *login.DelayedError
	switch {
	case errors.Is(err, login.ErrInvalidCredentials):
		return api.Reauthenticate403JSONResponse(apiresponse.Error("invalid_credentials", err)), nil
	case errors.Is(err, login.ErrNoPassword):
		return api.Reauthenticate403JSONResponse(apiresponse.Error("no_password", err)), nil
	case errors.Is(err, session.ErrInvalidSession):
		return api.Reauthenticate401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(apiresponse.Error("unauthorized", err))}, nil
	case errors.Is(err, login.ErrRateLimited):
		return reauthenticateTooOften("rate_limited", err, rateLimitRetrySeconds), nil
	case errors.As(err, &delayed):
		return reauthenticateTooOften("login_delayed", err, int(math.Ceil(delayed.RetryAfter.Seconds()))), nil
	case err != nil:
		return nil, err
	}
	return api.Reauthenticate204Response{}, nil
}

func reauthenticateTooOften(code string, err error, retryAfterSeconds int) api.Reauthenticate429JSONResponse {
	return api.Reauthenticate429JSONResponse{
		Body:    apiresponse.Error(code, err),
		Headers: api.Reauthenticate429ResponseHeaders{RetryAfter: &retryAfterSeconds},
	}
}

// recentPrincipal returns the request's principal, and whether it may make
// a change that needs a recent login (session.Principal.RequireRecent).
// When it may not, the caller answers 403 with reauthenticationRequired.
func (handler *Handler) recentPrincipal(ctx context.Context) (session.Principal, bool, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return session.Principal{}, false, errNoPrincipal
	}
	return principal, principal.RequireRecent(handler.dependencies.Clock.Now()) == nil, nil
}

// reauthenticationRequired is the body of every 403 that asks for a recent login.
func reauthenticationRequired() api.ReauthenticationRequiredJSONResponse {
	return api.ReauthenticationRequiredJSONResponse(apiresponse.Error("reauthentication_required", session.ErrReauthenticationRequired))
}
