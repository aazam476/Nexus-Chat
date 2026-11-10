package social

import (
	"context"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/social/store"
)

// Block blocks other: it ends any friendship and pending request between
// them, and stops friend requests and DMs in both directions. Blocking
// someone already blocked changes nothing.
func (service *Service) Block(ctx context.Context, me uuid.UUID, other uuid.UUID) error {
	if err := service.existing(ctx, me, other); err != nil {
		return err
	}
	if err := service.dependencies.Store.Block(ctx, store.BlockParams{Blocker: me, Blocked: other, Now: service.dependencies.Clock.Now()}); err != nil {
		return err
	}
	service.notify(ctx, change{me: me, mine: RelationshipBlocked, other: other, theirs: RelationshipNone})
	return nil
}

// Unblock lifts the caller's block on other.
func (service *Service) Unblock(ctx context.Context, me uuid.UUID, other uuid.UUID) error {
	removed, err := service.dependencies.Store.Unblock(ctx, store.UnblockParams{BlockerID: me, BlockedID: other})
	if err != nil {
		return err
	}
	if removed == 0 {
		return ErrNotFound
	}
	service.dependencies.Notifier.RelationshipChanged(ctx, me, other, RelationshipNone)
	return nil
}

// ListBlocked returns the users the caller blocked.
func (service *Service) ListBlocked(ctx context.Context, me uuid.UUID) ([]Contact, error) {
	rows, err := service.dependencies.Store.ListBlocked(ctx, me)
	if err != nil {
		return nil, err
	}
	blocked := make([]Contact, 0, len(rows))
	for _, row := range rows {
		blocked = append(blocked, Contact{UserID: row.ID, Username: row.Username, DisplayName: row.DisplayName, AvatarID: row.AvatarID, Since: row.Since})
	}
	return blocked, nil
}
