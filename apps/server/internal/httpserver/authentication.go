package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
)

// publicOperations lists the OpenAPI operations that need no session. Every
// other operation requires one: forgetting to protect a new endpoint fails
// closed. TestPublicOperationsMatchTheContract checks this list against the
// `security` declarations in api/openapi.yaml.
var publicOperations = map[string]bool{ //nolint:gochecknoglobals // constant lookup table
	"GetLiveness":            true,
	"GetReadiness":           true,
	"GetSetupStatus":         true,
	"GetInstanceInfo":        true,
	"CompleteSetup":          true,
	"Login":                  true,
	"Register":               true,
	"VerifyEmail":            true,
	"RequestPasswordReset":   true,
	"CompletePasswordReset":  true,
	"CompleteLoginMfa":       true,
	"BeginSecurityKeyLogin":  true,
	"FinishSecurityKeyLogin": true,
	"BeginPasskeyLogin":      true,
	"FinishPasskeyLogin":     true,
	"ListSignInProviders":    true,
	"BeginProviderSignIn":    true,
	"ProviderCallback":       true,
	"CompleteProviderSignIn": true,
	"GetAvatar":              true,
}

// Authenticator checks session tokens; *session.Service satisfies it.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (sessionstore.Session, error)
}

// requireSession is strict-server middleware that rejects requests to
// protected operations without a valid session, and stores the caller's
// Principal in the context for handlers.
func requireSession(authenticator Authenticator, logger *slog.Logger) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		if publicOperations[operationID] {
			return next
		}
		return func(ctx context.Context, writer http.ResponseWriter, request *http.Request, body any) (any, error) {
			token, ok := session.BearerToken(request.Header.Get("Authorization"))
			if !ok {
				writeUnauthorized(writer)
				return nil, nil
			}
			found, err := authenticator.Authenticate(ctx, token)
			if errors.Is(err, session.ErrInvalidSession) {
				writeUnauthorized(writer)
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			ctx = session.WithPrincipal(ctx, session.Principal{UserID: found.UserID, SessionID: found.ID, AuthenticatedAt: found.AuthenticatedAt})
			return next(ctx, writer, request, body)
		}
	}
}

func writeUnauthorized(writer http.ResponseWriter) {
	writer.Header().Set("WWW-Authenticate", `Bearer realm="nexus-chat"`)
	writeError(writer, http.StatusUnauthorized, api.Error{Code: "unauthorized", Message: "Log in to do this."})
}
