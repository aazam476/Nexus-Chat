package auth

import (
	"context"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
)

const (
	maximumDeviceNameLength = 64
	unknownDevice           = "Unknown device"
	rateLimitRetrySeconds   = 60
)

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// Handler serves the auth endpoints.
type Handler struct {
	logins   *login.Service
	sessions *session.Service
}

// NewHandler returns a Handler.
func NewHandler(logins *login.Service, sessions *session.Service) *Handler {
	return &Handler{logins: logins, sessions: sessions}
}

// Login answers POST /api/v1/auth/login.
func (handler *Handler) Login(ctx context.Context, request api.LoginRequestObject) (api.LoginResponseObject, error) {
	result, err := handler.logins.Login(ctx, login.Request{
		Login:     request.Body.Login,
		Password:  request.Body.Password,
		Device:    session.Device{Name: normalizeDeviceName(request.Body.DeviceName)},
		ClientKey: clientip.FromContext(ctx).Address.String(),
	})
	var delayed *login.DelayedError
	switch {
	case errors.Is(err, login.ErrInvalidCredentials):
		return api.Login401JSONResponse(api.Error{Code: "invalid_credentials", Message: err.Error()}), nil
	case errors.Is(err, login.ErrRateLimited):
		return tooManyAttempts("rate_limited", err, rateLimitRetrySeconds), nil
	case errors.As(err, &delayed):
		return tooManyAttempts("login_delayed", err, int(math.Ceil(delayed.RetryAfter.Seconds()))), nil
	case err != nil:
		return nil, err
	}
	return api.Login200JSONResponse{
		Token:   result.Token,
		Session: sessionResponse(result.Session, result.Session.ID),
		User:    accounts.CurrentUserResponse(result.User),
	}, nil
}

func tooManyAttempts(code string, err error, retryAfterSeconds int) api.Login429JSONResponse {
	return api.Login429JSONResponse{
		Body:    api.Error{Code: code, Message: err.Error()},
		Headers: api.Login429ResponseHeaders{RetryAfter: &retryAfterSeconds},
	}
}

// Logout answers POST /api/v1/auth/logout.
func (handler *Handler) Logout(ctx context.Context, _ api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	if err := handler.sessions.Revoke(ctx, principal.UserID, principal.SessionID); err != nil && !errors.Is(err, session.ErrNotFound) {
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
	sessions, err := handler.sessions.List(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	response := make(api.ListSessions200JSONResponse, 0, len(sessions))
	for _, found := range sessions {
		response = append(response, sessionResponse(found, principal.SessionID))
	}
	return response, nil
}

// RevokeSession answers DELETE /api/v1/auth/sessions/{sessionId}.
func (handler *Handler) RevokeSession(ctx context.Context, request api.RevokeSessionRequestObject) (api.RevokeSessionResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.sessions.Revoke(ctx, principal.UserID, request.SessionId)
	if errors.Is(err, session.ErrNotFound) {
		return api.RevokeSession404JSONResponse(api.Error{Code: "session_not_found", Message: err.Error()}), nil
	}
	if err != nil {
		return nil, err
	}
	return api.RevokeSession204Response{}, nil
}

func sessionResponse(found sessionstore.Session, currentSessionID uuid.UUID) api.Session {
	return api.Session{
		Id: found.ID, DeviceName: found.DeviceName, CreatedAt: found.CreatedAt,
		LastSeenAt: found.LastSeenAt, ExpiresAt: found.ExpiresAt, Current: found.ID == currentSessionID,
	}
}

// normalizeDeviceName trims the name and keeps it within 64 characters.
// A bad device name is not worth refusing a login over.
func normalizeDeviceName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || !utf8.ValidString(name) {
		return unknownDevice
	}
	if utf8.RuneCountInString(name) > maximumDeviceNameLength {
		name = string([]rune(name)[:maximumDeviceNameLength])
	}
	return name
}
