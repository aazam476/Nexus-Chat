package servers

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// ErrInviteInvalid means the invite does not exist, has expired or is used up.
var ErrInviteInvalid = errors.New("this invite does not exist, has expired, or has been used up")

// Invite limits: up to a year, up to 10,000 uses.
const (
	maximumInviteLifetime = 365 * 24 * time.Hour
	maximumInviteUses     = 10000
)

// Invite is an invite link's code and limits.
type Invite struct {
	Code      string
	ServerID  uuid.UUID
	CreatedBy *uuid.UUID
	MaxUses   *int32
	Uses      int32
	ExpiresAt *time.Time
	CreatedAt time.Time
}

// InviteLimits are an invite's optional limits.
type InviteLimits struct {
	MaxUses   *int32
	ExpiresIn *time.Duration
}

// valid reports whether the limits are within range.
func (limits InviteLimits) valid() bool {
	usesOK := limits.MaxUses == nil || (*limits.MaxUses >= 1 && *limits.MaxUses <= maximumInviteUses)
	lifetimeOK := limits.ExpiresIn == nil || (*limits.ExpiresIn > 0 && *limits.ExpiresIn <= maximumInviteLifetime)
	return usesOK && lifetimeOK
}

// newInviteCode is 10 random bytes as 16 lowercase base32 letters: 80 bits,
// too many to guess.
func newInviteCode() string {
	random := make([]byte, 10)
	_, _ = rand.Read(random) // crypto/rand never fails on supported platforms
	return lower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(random))
}

func lower(text string) string {
	out := []byte(text)
	for index, character := range out {
		if character >= 'A' && character <= 'Z' {
			out[index] = character + ('a' - 'A')
		}
	}
	return string(out)
}

// CreateInvite makes an invite link (CREATE_INVITES).
func (service *Service) CreateInvite(ctx context.Context, scope Scope, limits InviteLimits) (Invite, error) {
	a, err := service.load(ctx, scope)
	if err != nil {
		return Invite{}, err
	}
	if err := a.require(permissions.CreateInvites); err != nil {
		return Invite{}, err
	}
	if !limits.valid() {
		return Invite{}, ErrInvalid
	}
	now := service.clock.Now()
	params := store.InsertInviteParams{
		Code: newInviteCode(), ServerID: scope.ServerID, CreatedBy: &scope.UserID, MaxUses: limits.MaxUses, CreatedAt: now,
	}
	if limits.ExpiresIn != nil {
		expires := now.Add(*limits.ExpiresIn)
		params.ExpiresAt = &expires
	}
	if err := service.store.InsertInvite(ctx, params); err != nil {
		return Invite{}, err
	}
	return Invite{Code: params.Code, ServerID: scope.ServerID, CreatedBy: params.CreatedBy, MaxUses: params.MaxUses, ExpiresAt: params.ExpiresAt, CreatedAt: now}, nil
}

func inviteOf(row store.ServerInvite) Invite {
	return Invite{
		Code: row.Code, ServerID: row.ServerID, CreatedBy: row.CreatedBy, MaxUses: row.MaxUses,
		Uses: row.Uses, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}
}

// Invites lists the server's invites (MANAGE_SERVER).
func (service *Service) Invites(ctx context.Context, scope Scope) ([]Invite, error) {
	a, err := service.load(ctx, scope)
	if err != nil {
		return nil, err
	}
	if err := a.require(permissions.ManageServer); err != nil {
		return nil, err
	}
	rows, err := service.store.ListInvites(ctx, scope.ServerID)
	if err != nil {
		return nil, err
	}
	invites := make([]Invite, 0, len(rows))
	for _, row := range rows {
		invites = append(invites, inviteOf(row))
	}
	return invites, nil
}

// RevokeInvite deletes an invite; its creator or MANAGE_SERVER may.
func (service *Service) RevokeInvite(ctx context.Context, scope Scope, code string) error {
	a, err := service.load(ctx, scope)
	if err != nil {
		return err
	}
	invite, err := service.store.GetInvite(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && invite.ServerID != scope.ServerID) {
		return ErrInviteInvalid
	}
	if err != nil {
		return err
	}
	ownInvite := invite.CreatedBy != nil && *invite.CreatedBy == scope.UserID
	if !ownInvite && a.require(permissions.ManageServer) != nil {
		return ErrForbidden
	}
	_, err = service.store.DeleteInvite(ctx, store.DeleteInviteParams{Code: code, ServerID: scope.ServerID})
	return err
}

// Preview is what an invite shows before joining.
type Preview struct {
	Server  Server
	Members int64
}

// usable returns the invite if it can still be used.
func (service *Service) usable(ctx context.Context, code string) (store.ServerInvite, error) {
	invite, err := service.store.GetInvite(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ServerInvite{}, ErrInviteInvalid
	}
	if err != nil {
		return store.ServerInvite{}, err
	}
	expired := invite.ExpiresAt != nil && !invite.ExpiresAt.After(service.clock.Now())
	usedUp := invite.MaxUses != nil && invite.Uses >= *invite.MaxUses
	if expired || usedUp {
		return store.ServerInvite{}, ErrInviteInvalid
	}
	return invite, nil
}

// PreviewInvite shows which server an invite is for, to anyone signed in.
func (service *Service) PreviewInvite(ctx context.Context, code string) (Preview, error) {
	invite, err := service.usable(ctx, code)
	if err != nil {
		return Preview{}, err
	}
	server, err := service.store.GetServer(ctx, invite.ServerID)
	if err != nil {
		return Preview{}, err
	}
	members, err := service.store.CountServerMembers(ctx, invite.ServerID)
	if err != nil {
		return Preview{}, err
	}
	return Preview{Server: serverOf(server), Members: members}, nil
}

// Join adds userID to the server an invite is for, using the invite once.
// Joining a server one is already in changes nothing and uses nothing.
func (service *Service) Join(ctx context.Context, code string, userID uuid.UUID) (uuid.UUID, error) {
	invite, err := service.usable(ctx, code)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := service.load(ctx, Scope{ServerID: invite.ServerID, UserID: userID}); err == nil {
		return invite.ServerID, nil
	}
	serverID, err := service.store.UseInvite(ctx, store.UseInviteParams{Code: code, Now: service.clock.Now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrInviteInvalid
	}
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := service.store.AddServerMember(ctx, store.AddServerMemberParams{ServerID: serverID, UserID: userID, JoinedAt: service.clock.Now()}); err != nil {
		return uuid.Nil, err
	}
	return serverID, service.notify(ctx, serverID)
}
