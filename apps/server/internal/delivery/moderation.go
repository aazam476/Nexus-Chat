package delivery

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
)

// Moderators delete messages by deleting their ciphertext (spec §4.7): the
// archive copy and any live copies still waiting. A signed directive tells
// members' devices, which drop their decrypted copy.

// ErrNoMessage means the channel has no message with that ID.
var ErrNoMessage = errors.New("there is no such message in this channel")

// DeletedMessage is a channel message a moderator deleted.
type DeletedMessage struct {
	MessageID uuid.UUID
	// SenderID is nil once the sender's account is gone.
	SenderID *uuid.UUID
}

// DeleteChannelMessage deletes another member's message from a channel
// (MANAGE_MESSAGES there). Senders delete their own with an encrypted
// deletion instead, which needs no permission.
func (service *Service) DeleteChannelMessage(ctx context.Context, ref ChannelRef, messageID uuid.UUID) error {
	grant, err := service.dependencies.Channels.ChannelGrant(ctx, ref)
	if err != nil {
		return err
	}
	if !grant.MayManageMessages {
		return ErrNotAllowed
	}
	row, err := service.dependencies.Store.DeleteChannelMessage(ctx, store.DeleteChannelArchiveParams{
		ConversationID: grant.AudienceID, MessageID: messageID, ChannelID: &ref.ChannelID,
	})
	if errors.Is(err, store.ErrNoMessage) {
		return ErrNoMessage
	}
	if err != nil {
		return err
	}
	directive := Directive{Type: DirectiveDelete, Reason: "moderator", MessageID: &messageID, ChannelID: &ref.ChannelID}
	if row.SenderID != nil {
		directive.UserID = *row.SenderID
	}
	if err := service.publish(ctx, grant.AudienceID, directive); err != nil {
		return err
	}
	return service.dependencies.Channels.MessageDeleted(ctx, ref, DeletedMessage{MessageID: messageID, SenderID: row.SenderID})
}

// PurgeMessages deletes everything userID sent to a server's audiences
// since a time, as a ban asks, and tells their members. It returns how
// many messages went.
func (service *Service) PurgeMessages(ctx context.Context, audienceIDs []uuid.UUID, userID uuid.UUID, since time.Time) (int64, error) {
	purged, err := service.dependencies.Store.Purge(ctx, audienceIDs, userID, since)
	if err != nil {
		return 0, err
	}
	for _, audienceID := range audienceIDs {
		if err := service.publish(ctx, audienceID, Directive{Type: DirectivePurge, UserID: userID, Reason: "ban", Since: &since}); err != nil {
			return purged, err
		}
	}
	return purged, nil
}
