// Package passwordreset lets a user who forgot their password choose a new
// one through a link sent to their email (spec §4.1).
//
// A reset restores login only. Encryption keys are recovered separately with
// the recovery phrase or another device (spec §5.4); the email says so.
//
// Requesting a reset always looks the same to the requester, whether or not
// the address has an account, so the form cannot be used to find out who
// is registered. Start reading at [Service.Request].
package passwordreset

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// ErrRateLimited means too many reset requests came from one address.
var ErrRateLimited = errors.New("too many password reset requests; try again later")

type accountStore interface {
	GetUserByEmail(ctx context.Context, email string) (accountsstore.User, error)
	SetPassword(ctx context.Context, arg accountsstore.SetPasswordParams) error
	MarkEmailVerified(ctx context.Context, arg accountsstore.MarkEmailVerifiedParams) error
}

type sessionRevoker interface {
	DeleteSessionsForUser(ctx context.Context, userID uuid.UUID) error
}

type tokenRedeemer interface {
	Redeem(ctx context.Context, purpose tokens.Purpose, token string) (uuid.UUID, error)
}

type resetMailer interface {
	SendPasswordReset(ctx context.Context, user accountsstore.User) error
}

type passwordChecker interface {
	CheckPassword(password string) error
}

type passwordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
}

type attemptLimiter interface {
	Allow(key string) bool
}

// Dependencies are the collaborators a Service needs.
type Dependencies struct {
	Accounts  accountStore
	Sessions  sessionRevoker
	Tokens    tokenRedeemer
	Mailer    resetMailer
	Validator passwordChecker
	Hasher    passwordHasher
	Limiter   attemptLimiter
	Clock     clock.Clock
	Logger    *slog.Logger
}

// Service handles password resets.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

// Request emails a reset link to email if it belongs to an account. Its
// only possible error is ErrRateLimited; every other problem is logged and
// hidden, so callers cannot learn whether the address has an account.
func (service *Service) Request(ctx context.Context, email string, clientKey string) error {
	if !service.dependencies.Limiter.Allow(clientKey) {
		return ErrRateLimited
	}
	user, err := service.dependencies.Accounts.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err == nil {
		err = service.dependencies.Mailer.SendPasswordReset(ctx, user)
	}
	if err != nil {
		service.dependencies.Logger.WarnContext(ctx, "password reset email not sent", slog.Any("error", err))
	}
	return nil
}

// Complete sets a new password using the token from the email link, marks
// the email verified (the user just proved they read it), and logs out
// every session, so whoever knew the old password is locked out.
func (service *Service) Complete(ctx context.Context, token string, newPassword string) error {
	if err := service.dependencies.Validator.CheckPassword(newPassword); err != nil {
		var problems validation.Errors
		problems.Add("password", err)
		return problems.Err()
	}
	userID, err := service.dependencies.Tokens.Redeem(ctx, tokens.ResetPassword, token)
	if err != nil {
		return err
	}
	hash, err := service.dependencies.Hasher.Hash(ctx, newPassword)
	if err != nil {
		return err
	}
	accounts := service.dependencies.Accounts
	if err := accounts.SetPassword(ctx, accountsstore.SetPasswordParams{ID: userID, PasswordHash: &hash}); err != nil {
		return fmt.Errorf("saving new password: %w", err)
	}
	now := service.dependencies.Clock.Now()
	if err := accounts.MarkEmailVerified(ctx, accountsstore.MarkEmailVerifiedParams{ID: userID, EmailVerifiedAt: &now}); err != nil {
		return fmt.Errorf("marking email verified: %w", err)
	}
	if err := service.dependencies.Sessions.DeleteSessionsForUser(ctx, userID); err != nil {
		return fmt.Errorf("logging out old sessions: %w", err)
	}
	return nil
}
