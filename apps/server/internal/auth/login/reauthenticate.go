package login

import (
	"context"
	"errors"
	"fmt"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

// ErrNoPassword means the account signs in only with a provider or a
// passkey, so it confirms who it is by logging in again.
var ErrNoPassword = errors.New("this account has no password; log in again to make this change")

// Reauthenticate checks the password of principal's account and, if it is
// right, marks the session as recently authenticated (see
// session.Principal.RequireRecent). Wrong passwords count towards the same
// per-account delays as Login, so this is no way around them.
func (service *Service) Reauthenticate(ctx context.Context, principal session.Principal, password string, clientKey string) error {
	if !service.dependencies.Limiter.Allow(clientKey) {
		return ErrRateLimited
	}
	user, err := service.dependencies.Accounts.GetUserByID(ctx, principal.UserID)
	if err != nil {
		return fmt.Errorf("looking up account: %w", err)
	}
	if user.PasswordHash == nil {
		return ErrNoPassword
	}
	now := service.dependencies.Clock.Now()
	if user.LoginBlockedUntil != nil && now.Before(*user.LoginBlockedUntil) {
		return &DelayedError{RetryAfter: user.LoginBlockedUntil.Sub(now)}
	}
	if err := service.checkPassword(ctx, user, password, now); err != nil {
		return err
	}
	return service.dependencies.Sessions.MarkAuthenticated(ctx, principal)
}
