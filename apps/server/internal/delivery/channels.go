package delivery

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
)

// A server channel's messages travel in its audience's MLS group
// (docs/e2ee-design.md §3.1), which is a conversation of kind "audience".
// Each application message names its channel, so the server can check
// SEND_MESSAGES there and members can tell the channels apart.

// ChannelRef is a server channel and the user asking about it.
type ChannelRef struct {
	ServerID  uuid.UUID
	ChannelID uuid.UUID
	UserID    uuid.UUID
}

// ChannelGrant is what the user may do with the channel's messages.
type ChannelGrant struct {
	// AudienceID is the conversation of the channel's audience.
	AudienceID uuid.UUID
	MaySend    bool
	// HistorySince is the oldest archive copy the user may fetch; zero
	// when the channel lets them read history from before they joined.
	HistorySince time.Time
}

// ChannelAccess answers for server channels. It returns ErrNotFound when
// the user cannot see the channel or it is not a text channel.
type ChannelAccess interface {
	ChannelGrant(ctx context.Context, ref ChannelRef) (ChannelGrant, error)
}

// SendToChannel accepts a message for a server channel, in the current
// epoch of its audience's group.
func (service *Service) SendToChannel(ctx context.Context, ref ChannelRef, outgoing Outgoing) (int64, error) {
	grant, err := service.dependencies.Channels.ChannelGrant(ctx, ref)
	if err != nil {
		return 0, err
	}
	if !grant.MaySend {
		return 0, ErrNotAllowed
	}
	outgoing.channelID = &ref.ChannelID
	return service.send(ctx, ref.UserID, grant.AudienceID, outgoing)
}

// ChannelArchived is an archive copy of a channel's message, with the
// MLS group whose exporter derives its history key.
type ChannelArchived struct {
	MessageID  uuid.UUID
	MLSGroupID []byte
	Epoch      int64
	SenderID   *uuid.UUID
	Sealed     []byte
	CreatedAt  time.Time
}

// ChannelArchive returns a channel's archive copies made before
// page.Before, newest first, back to where the user may read.
func (service *Service) ChannelArchive(ctx context.Context, ref ChannelRef, page ArchivePage) ([]ChannelArchived, error) {
	grant, err := service.dependencies.Channels.ChannelGrant(ctx, ref)
	if err != nil {
		return nil, err
	}
	rows, err := service.dependencies.Store.ChannelArchiveBefore(ctx, store.ChannelArchiveBeforeParams{
		ChannelID: &ref.ChannelID, Before: page.Before, Since: grant.HistorySince, RowLimit: pageSize(page.Limit),
	})
	if err != nil {
		return nil, err
	}
	archived := make([]ChannelArchived, 0, len(rows))
	for _, row := range rows {
		archived = append(archived, ChannelArchived{
			MessageID: row.MessageID, MLSGroupID: row.MlsGroupID, Epoch: row.Epoch, SenderID: row.SenderID, Sealed: row.Sealed, CreatedAt: row.CreatedAt,
		})
	}
	return archived, nil
}
