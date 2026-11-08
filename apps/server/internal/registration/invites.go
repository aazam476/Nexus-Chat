package registration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

type inviteStore interface {
	InsertInvite(ctx context.Context, arg store.InsertInviteParams) error
	ListInvites(ctx context.Context) ([]store.ListInvitesRow, error)
	DeleteInvite(ctx context.Context, id uuid.UUID) (int64, error)
}

// InviteOptions limit an invite. Zero values mean "no limit".
type InviteOptions struct {
	MaxUses   int32
	ExpiresAt *time.Time
}

// Invites creates and manages instance invite codes.
type Invites struct {
	store inviteStore
	clock clock.Clock
}

// NewInvites returns an Invites service.
func NewInvites(invites inviteStore, clock clock.Clock) *Invites {
	return &Invites{store: invites, clock: clock}
}

// Create makes a new invite and returns its code. The code is shown once;
// only its hash is stored.
func (invites *Invites) Create(ctx context.Context, createdBy uuid.UUID, options InviteOptions) (string, uuid.UUID, error) {
	code := formatInviteCode(rand.Text())
	hash := HashInviteCode(code)
	params := store.InsertInviteParams{
		ID: uuid.Must(uuid.NewV7()), CodeHash: hash[:], CreatedBy: &createdBy,
		ExpiresAt: options.ExpiresAt, CreatedAt: invites.clock.Now(),
	}
	if options.MaxUses > 0 {
		params.MaxUses = &options.MaxUses
	}
	if err := invites.store.InsertInvite(ctx, params); err != nil {
		return "", uuid.Nil, fmt.Errorf("saving invite: %w", err)
	}
	return code, params.ID, nil
}

// List returns every invite, newest first.
func (invites *Invites) List(ctx context.Context) ([]store.ListInvitesRow, error) {
	rows, err := invites.store.ListInvites(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing invites: %w", err)
	}
	return rows, nil
}

// Delete revokes an invite. It reports false if there was none with id.
func (invites *Invites) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	deleted, err := invites.store.DeleteInvite(ctx, id)
	if err != nil {
		return false, fmt.Errorf("deleting invite: %w", err)
	}
	return deleted > 0, nil
}

// formatInviteCode groups a 26-character code as XXXX-XXXX-...-XX.
func formatInviteCode(raw string) string {
	var groups []string
	for start := 0; start < len(raw); start += 4 {
		groups = append(groups, raw[start:min(start+4, len(raw))])
	}
	return strings.Join(groups, "-")
}

// HashInviteCode normalizes (upper case, no dashes or spaces) and hashes a
// code, so people can type it however they like.
func HashInviteCode(code string) [sha256.Size]byte {
	normalized := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	return sha256.Sum256([]byte(normalized))
}
