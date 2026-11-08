// Package tokens issues and redeems single-use account tokens: the links
// sent by email to verify an address or reset a password.
//
// Like session tokens, they are 256 random bits; only their SHA-256 hash is
// stored. Redeeming a token deletes it, so each works once. Issuing a new
// token for the same purpose cancels the user's older ones. Start reading
// at [Service.Issue].
package tokens

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// Purpose says what a token may be used for. A token issued for one purpose
// never works for another.
type Purpose string

// Token purposes and how long each stays valid.
const (
	VerifyEmail   Purpose = "verify_email"
	ResetPassword Purpose = "reset_password"
	// MFALogin is a login that passed the password check and still needs a
	// second factor.
	MFALogin Purpose = "mfa_login"
)

// lifetimes: a reset link is short-lived because it grants account access.
var lifetimes = map[Purpose]time.Duration{ //nolint:gochecknoglobals // constant lookup table
	VerifyEmail:   48 * time.Hour,
	ResetPassword: time.Hour,
	MFALogin:      5 * time.Minute,
}

// ErrInvalidToken means the token is unknown, already used, expired, or for
// another purpose. The cases are not told apart, to reveal nothing.
var ErrInvalidToken = errors.New("this link is invalid or has expired; request a new one")

type tokenStore interface {
	InsertToken(ctx context.Context, arg store.InsertTokenParams) error
	ConsumeToken(ctx context.Context, arg store.ConsumeTokenParams) (uuid.UUID, error)
	DeleteTokensForUser(ctx context.Context, arg store.DeleteTokensForUserParams) error
}

// Service issues and redeems tokens.
type Service struct {
	store tokenStore
	clock clock.Clock
}

// NewService returns a Service.
func NewService(tokens tokenStore, clock clock.Clock) *Service {
	return &Service{store: tokens, clock: clock}
}

// Issue creates a token for userID and purpose, replacing older ones.
func (service *Service) Issue(ctx context.Context, userID uuid.UUID, purpose Purpose) (string, error) {
	lifetime, known := lifetimes[purpose]
	if !known {
		return "", fmt.Errorf("unknown token purpose %q", purpose)
	}
	if err := service.store.DeleteTokensForUser(ctx, store.DeleteTokensForUserParams{UserID: userID, Purpose: string(purpose)}); err != nil {
		return "", fmt.Errorf("cancelling older tokens: %w", err)
	}
	secret := make([]byte, 32)
	_, _ = rand.Read(secret) // crypto/rand.Read never fails (Go 1.24+)
	token := base64.RawURLEncoding.EncodeToString(secret)
	now := service.clock.Now()
	err := service.store.InsertToken(ctx, store.InsertTokenParams{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, Purpose: string(purpose),
		TokenHash: hash(token), CreatedAt: now, ExpiresAt: now.Add(lifetime),
	})
	if err != nil {
		return "", fmt.Errorf("saving token: %w", err)
	}
	return token, nil
}

// Redeem uses up token and returns the user it was issued to.
func (service *Service) Redeem(ctx context.Context, purpose Purpose, token string) (uuid.UUID, error) {
	userID, err := service.store.ConsumeToken(ctx, store.ConsumeTokenParams{
		TokenHash: hash(token), Purpose: string(purpose), Now: service.clock.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrInvalidToken
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("redeeming token: %w", err)
	}
	return userID, nil
}

func hash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
