package servers

import (
	"context"
	"errors"

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
