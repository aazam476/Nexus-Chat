package delivery

import (
	"context"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// ChannelReadState is how far a member has read a server text channel.
type ChannelReadState struct {
	ChannelID  uuid.UUID
	LastReadID int64
	// UnreadCount counts application messages from others after
	// LastReadID, up to 1000.
	UnreadCount int64
}

// MarkChannelRead moves the caller's read position in a channel forward
// to lastReadID (never back, never past the channel's newest message) and
// returns where it now is. A negative lastReadID is a *validation.Error.
func (service *Service) MarkChannelRead(ctx context.Context, ref ChannelRef, lastReadID int64) (int64, error) {
	if lastReadID < 0 {
		var problems validation.Errors
		problems.Add("lastReadId", errNegativeReadPosition)
		return 0, problems.Err()
	}
	if _, err := service.dependencies.Channels.ChannelGrant(ctx, ref); err != nil {
		return 0, err
	}
	position, err := service.dependencies.Store.MarkChannelRead(ctx, store.MarkChannelReadParams{
		ChannelID: ref.ChannelID, UserID: ref.UserID, LastReadID: lastReadID, Now: service.dependencies.Clock.Now(),
	})
	if err != nil {
		return 0, err
	}
	service.dependencies.Notifier.ChannelReadChanged(ctx, ref.UserID, ref.ChannelID, position)
	return position, nil
}

// ChannelReadStates returns the caller's read state in each text channel
// of the server they can see.
func (service *Service) ChannelReadStates(ctx context.Context, serverID uuid.UUID, userID uuid.UUID) ([]ChannelReadState, error) {
	channels, err := service.dependencies.Channels.TextChannels(ctx, serverID, userID)
	if err != nil {
		return nil, err
	}
	rows, err := service.dependencies.Store.ChannelReadStates(ctx, store.ChannelReadStatesParams{UserID: userID, ChannelIds: channels})
	if err != nil {
		return nil, err
	}
	states := make([]ChannelReadState, 0, len(rows))
	for _, row := range rows {
		states = append(states, ChannelReadState(row))
	}
	return states, nil
}
