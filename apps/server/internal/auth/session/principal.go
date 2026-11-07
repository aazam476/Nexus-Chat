package session

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// Principal identifies who sent an authenticated request.
type Principal struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
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
