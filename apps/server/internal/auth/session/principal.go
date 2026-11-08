package session

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RecentAuthentication is how long after logging in, or confirming the
// password, a session may change how its account logs in.
const RecentAuthentication = 10 * time.Minute

// ErrReauthenticationRequired means the change needs a recent login.
var ErrReauthenticationRequired = errors.New("confirm your password or log in again to make this change")

// Principal identifies who sent an authenticated request.
type Principal struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	// AuthenticatedAt is when the session's user last proved who they are.
	AuthenticatedAt time.Time
}

// RequireRecent returns ErrReauthenticationRequired unless the principal
// authenticated within RecentAuthentication of now. Handlers call it before
// changes a stolen session must not make: adding or removing ways to log
// in, and replacing backup codes.
func (principal Principal) RequireRecent(now time.Time) error {
	if now.Sub(principal.AuthenticatedAt) > RecentAuthentication {
		return ErrReauthenticationRequired
	}
	return nil
}

type principalKey struct{}

// WithPrincipal returns a context carrying principal.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// PrincipalFrom returns the request's principal. ok is false for requests
// that were not authenticated, which only public operations receive.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

// BearerToken extracts the token from an "Authorization: Bearer <token>"
// header value. The scheme name is case-insensitive (RFC 9110 §11.1).
func BearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}
