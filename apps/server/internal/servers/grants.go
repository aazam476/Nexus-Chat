package servers

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
)

// ChannelGrant says what ref.UserID may do with a text channel's messages
// (it implements delivery.ChannelAccess). Someone who cannot see the
// channel, or a channel that is not a text channel, is delivery.ErrNotFound.
func (service *Service) ChannelGrant(ctx context.Context, ref delivery.ChannelRef) (delivery.ChannelGrant, error) {
	a, err := service.load(ctx, Scope{ServerID: ref.ServerID, UserID: ref.UserID})
	if errors.Is(err, ErrNotFound) {
		return delivery.ChannelGrant{}, delivery.ErrNotFound
	}
	if err != nil {
		return delivery.ChannelGrant{}, err
	}
	channel, err := service.channelIn(ctx, ref.ServerID, ref.ChannelID)
	if errors.Is(err, ErrNoChannel) {
		return delivery.ChannelGrant{}, delivery.ErrNotFound
	}
	if err != nil {
		return delivery.ChannelGrant{}, err
	}
	mine := a.inChannel(channel.ID)
	if channel.AudienceID == nil || !mine.Has(permissions.ViewChannel) {
		return delivery.ChannelGrant{}, delivery.ErrNotFound
	}
	grant := delivery.ChannelGrant{AudienceID: *channel.AudienceID, MaySend: mine.Has(permissions.SendMessages)}
	if !mine.Has(permissions.ReadHistoryBeforeJoin) {
		grant.HistorySince = a.joinedAt
	}
	return grant, nil
}

// ErrCannotType means the user may not send in the channel, so nobody
// hears them type there.
var ErrCannotType = errors.New("you cannot send in this channel")

// ChannelTypingRecipients returns who hears that userID is typing in a
// text channel: everyone else in its audience's group. The user must be
// able to send there (it implements the channel half of realtime.TypingRelay).
func (service *Service) ChannelTypingRecipients(ctx context.Context, channelID uuid.UUID, userID uuid.UUID) ([]uuid.UUID, error) {
	channel, err := service.store.GetChannel(ctx, channelID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoChannel
	}
	if err != nil {
		return nil, err
	}
	a, err := service.load(ctx, Scope{ServerID: channel.ServerID, UserID: userID})
	if err != nil {
		return nil, err
	}
	if channel.AudienceID == nil || !a.inChannel(channel.ID).Has(permissions.SendMessages) {
		return nil, ErrCannotType
	}
	members, err := service.store.AudienceMemberIDs(ctx, *channel.AudienceID)
	if err != nil {
		return nil, err
	}
	others := make([]uuid.UUID, 0, len(members))
	for _, member := range members {
		if member != userID {
			others = append(others, member)
		}
	}
	return others, nil
}

// TextChannels lists the server's text channels the user can see (it
// implements the rest of delivery.ChannelAccess).
func (service *Service) TextChannels(ctx context.Context, serverID uuid.UUID, userID uuid.UUID) ([]uuid.UUID, error) {
	a, err := service.load(ctx, Scope{ServerID: serverID, UserID: userID})
	if errors.Is(err, ErrNotFound) {
		return nil, delivery.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	channels, err := service.store.ListChannels(ctx, serverID)
	if err != nil {
		return nil, err
	}
	visible := []uuid.UUID{}
	for _, channel := range channels {
		if channel.AudienceID != nil && a.inChannel(channel.ID).Has(permissions.ViewChannel) {
			visible = append(visible, channel.ID)
		}
	}
	return visible, nil
}
