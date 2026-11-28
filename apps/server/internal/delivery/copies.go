package delivery

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/conversations"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
)

// A report about a message (spec §4.7) needs its stored archive copy, so
// the instance can sign a receipt for it. Only someone who could read the
// message may ask.

// StoredCopy is one stored archive copy.
type StoredCopy struct {
	ConversationID uuid.UUID
	// ChannelID is set for a server channel's message.
	ChannelID *uuid.UUID
	MessageID uuid.UUID
	Epoch     int64
	// SenderID is nil once the sender's account is gone.
	SenderID  *uuid.UUID
	Sealed    []byte
	CreatedAt time.Time
	// MLSGroupID is the group whose exporter derives its history key.
	MLSGroupID []byte
	// ThreadRootID is set for a reply in a thread.
	ThreadRootID *uuid.UUID
}

// ChannelCopy returns a channel message's archive copy to someone who can see the channel.
func (service *Service) ChannelCopy(ctx context.Context, ref ChannelRef, messageID uuid.UUID) (StoredCopy, error) {
	grant, err := service.dependencies.Channels.ChannelGrant(ctx, ref)
	if err != nil {
		return StoredCopy{}, err
	}
	found, err := service.storedCopy(ctx, grant.AudienceID, messageID)
	if err != nil {
		return StoredCopy{}, err
	}
	if found.ChannelID == nil || *found.ChannelID != ref.ChannelID || found.CreatedAt.Before(grant.HistorySince) {
		return StoredCopy{}, ErrNoMessage
	}
	return found, nil
}

// ConversationCopy returns a direct or group message's archive copy to a member.
func (service *Service) ConversationCopy(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, messageID uuid.UUID) (StoredCopy, error) {
	group, err := service.group(ctx, me, conversationID)
	if err != nil {
		return StoredCopy{}, err
	}
	if group.kind == conversations.KindAudience {
		return StoredCopy{}, ErrNotFound
	}
	return service.storedCopy(ctx, conversationID, messageID)
}

func (service *Service) storedCopy(ctx context.Context, conversationID uuid.UUID, messageID uuid.UUID) (StoredCopy, error) {
	row, err := service.dependencies.Store.GetArchivedCopy(ctx, store.GetArchivedCopyParams{ConversationID: conversationID, MessageID: messageID})
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredCopy{}, ErrNoMessage
	}
	if err != nil {
		return StoredCopy{}, err
	}
	group, err := service.state(ctx, conversationID)
	if err != nil {
		return StoredCopy{}, err
	}
	return StoredCopy{
		ConversationID: row.ConversationID, ChannelID: row.ChannelID, MessageID: row.MessageID, Epoch: row.Epoch,
		SenderID: row.SenderID, Sealed: row.Sealed, CreatedAt: row.CreatedAt, MLSGroupID: group.GroupID, ThreadRootID: row.ThreadRootID,
	}, nil
}
