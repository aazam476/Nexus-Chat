package identities

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/identities/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// selfSigningLabel names the statement the master key signs; it must match
// packages/crypto/src/identity/identity.ts.
const selfSigningLabel = "nexus-chat self-signing key v1"

// Errors returned to the caller.
var (
	ErrNotFound         = errors.New("this user has not published an identity")
	ErrInvalidIdentity  = errors.New("the keys must be 32 bytes and the signature 64")
	ErrInvalidSignature = errors.New("the master key did not sign this self-signing key")
)

// Identity is a user's published identity.
type Identity struct {
	MasterPublicKey      []byte
	SelfSigningPublicKey []byte
	SelfSigningSignature []byte
}

type identityStore interface {
	GetIdentity(ctx context.Context, userID uuid.UUID) (store.UserIdentity, error)
	PutIdentity(ctx context.Context, params store.PutIdentityParams) error
}

// Service publishes and looks up identities.
type Service struct {
	store identityStore
	clock clock.Clock
}

// NewService returns a Service.
func NewService(identities identityStore, clock clock.Clock) *Service {
	return &Service{store: identities, clock: clock}
}

// statement is the label, then the self-signing key, each with a 4-byte
// big-endian length, as the client signs it.
func statement(selfSigningKey []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(selfSigningLabel)))
	out = append(out, selfSigningLabel...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(selfSigningKey))) //nolint:gosec // 32 bytes, checked first
	return append(out, selfSigningKey...)
}

// Publish stores the caller's identity. Replacing a different master key
// needs a recent login; a new self-signing key under the same master key
// (after losing a device) does not.
func (service *Service) Publish(ctx context.Context, caller session.Principal, identity Identity) error {
	if len(identity.MasterPublicKey) != ed25519.PublicKeySize ||
		len(identity.SelfSigningPublicKey) != ed25519.PublicKeySize ||
		len(identity.SelfSigningSignature) != ed25519.SignatureSize {
		return ErrInvalidIdentity
	}
	if !ed25519.Verify(identity.MasterPublicKey, statement(identity.SelfSigningPublicKey), identity.SelfSigningSignature) {
		return ErrInvalidSignature
	}
	current, err := service.Get(ctx, caller.UserID)
	switch {
	case errors.Is(err, ErrNotFound):
	case err != nil:
		return err
	case !bytes.Equal(current.MasterPublicKey, identity.MasterPublicKey):
		if err := caller.RequireRecent(service.clock.Now()); err != nil {
			return err
		}
	}
	return service.store.PutIdentity(ctx, store.PutIdentityParams{
		UserID:               caller.UserID,
		MasterPublicKey:      identity.MasterPublicKey,
		SelfSigningPublicKey: identity.SelfSigningPublicKey,
		SelfSigningSignature: identity.SelfSigningSignature,
		UpdatedAt:            service.clock.Now(),
	})
}

// Get returns a user's published identity.
func (service *Service) Get(ctx context.Context, userID uuid.UUID) (Identity, error) {
	row, err := service.store.GetIdentity(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, ErrNotFound
	}
	if err != nil {
		return Identity{}, err
	}
	return Identity{
		MasterPublicKey:      row.MasterPublicKey,
		SelfSigningPublicKey: row.SelfSigningPublicKey,
		SelfSigningSignature: row.SelfSigningSignature,
	}, nil
}
