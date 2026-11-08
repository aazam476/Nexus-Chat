package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// Session lifetime rules.
const (
	// Lifetime is how long a session lasts without being used.
	Lifetime = 90 * 24 * time.Hour
	// touchInterval limits how often activity is written to the database:
	// at most once per session per interval, not on every request.
	touchInterval = 5 * time.Minute
	// tokenPrefix makes tokens recognizable to secret scanners if one is
	// ever pasted somewhere it should not be.
	tokenPrefix = "nxs_"
)

// ErrInvalidSession means the token is unknown, malformed or expired.
var ErrInvalidSession = errors.New("the session is not valid; log in again")

// ErrNotFound means the session does not exist or belongs to another user.
var ErrNotFound = errors.New("session not found")

// sessionStore is the subset of the generated queries the service uses.
type sessionStore interface {
	InsertSession(ctx context.Context, arg store.InsertSessionParams) error
	GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (store.Session, error)
	TouchSession(ctx context.Context, arg store.TouchSessionParams) error
	ListSessionsForUser(ctx context.Context, arg store.ListSessionsForUserParams) ([]store.Session, error)
	DeleteSession(ctx context.Context, arg store.DeleteSessionParams) (int64, error)
	MarkSessionAuthenticated(ctx context.Context, arg store.MarkSessionAuthenticatedParams) (int64, error)
}

// Device describes the client creating a session, as shown in the session
// list.
type Device struct {
	Name      string
	UserAgent string
}

// Service creates and checks sessions.
type Service struct {
	store sessionStore
	clock clock.Clock
}

// NewService returns a Service.
func NewService(sessions sessionStore, clock clock.Clock) *Service {
	return &Service{store: sessions, clock: clock}
}

// Create starts a session for userID and returns its token, which must be
// given to the client and is never stored.
func (service *Service) Create(ctx context.Context, userID uuid.UUID, device Device) (string, store.Session, error) {
	secret := make([]byte, 32)
	_, _ = rand.Read(secret) // crypto/rand.Read never fails (Go 1.24+)
	token := tokenPrefix + base64.RawURLEncoding.EncodeToString(secret)
	now := service.clock.Now()
	session := store.Session{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, TokenHash: hashToken(token),
		DeviceName: device.Name, UserAgent: device.UserAgent,
		CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(Lifetime), AuthenticatedAt: now,
	}
	err := service.store.InsertSession(ctx, store.InsertSessionParams(session))
	if err != nil {
		return "", store.Session{}, fmt.Errorf("saving session: %w", err)
	}
	return token, session, nil
}

// Authenticate returns the session for token, extending it if it was last
// used more than a few minutes ago.
func (service *Service) Authenticate(ctx context.Context, token string) (store.Session, error) {
	if !strings.HasPrefix(token, tokenPrefix) {
		return store.Session{}, ErrInvalidSession
	}
	session, err := service.store.GetSessionByTokenHash(ctx, hashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Session{}, ErrInvalidSession
	}
	if err != nil {
		return store.Session{}, fmt.Errorf("looking up session: %w", err)
	}
	now := service.clock.Now()
	if !now.Before(session.ExpiresAt) {
		return store.Session{}, ErrInvalidSession
	}
	if now.Sub(session.LastSeenAt) >= touchInterval {
		session.LastSeenAt, session.ExpiresAt = now, now.Add(Lifetime)
		err := service.store.TouchSession(ctx, store.TouchSessionParams{ID: session.ID, LastSeenAt: now, ExpiresAt: session.ExpiresAt})
		if err != nil {
			return store.Session{}, fmt.Errorf("recording session activity: %w", err)
		}
	}
	return session, nil
}

// MarkAuthenticated records that the user of principal's session just
// confirmed their password.
func (service *Service) MarkAuthenticated(ctx context.Context, principal Principal) error {
	updated, err := service.store.MarkSessionAuthenticated(ctx, store.MarkSessionAuthenticatedParams{
		ID: principal.SessionID, AuthenticatedAt: service.clock.Now(), UserID: principal.UserID,
	})
	if err != nil {
		return fmt.Errorf("recording authentication: %w", err)
	}
	if updated == 0 {
		return ErrInvalidSession // revoked in the meantime
	}
	return nil
}

// List returns the user's active sessions, most recently used first.
func (service *Service) List(ctx context.Context, userID uuid.UUID) ([]store.Session, error) {
	sessions, err := service.store.ListSessionsForUser(ctx, store.ListSessionsForUserParams{UserID: userID, Now: service.clock.Now()})
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	return sessions, nil
}

// Revoke ends one of the user's sessions. Users can revoke only their own.
func (service *Service) Revoke(ctx context.Context, userID uuid.UUID, sessionID uuid.UUID) error {
	deleted, err := service.store.DeleteSession(ctx, store.DeleteSessionParams{ID: sessionID, UserID: userID})
	if err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

func hashToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}
