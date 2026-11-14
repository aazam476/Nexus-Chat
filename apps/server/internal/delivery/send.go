package delivery

import (
	"context"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/conversations"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/mlsheader"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
)

// Commit is a commit and what travels with it.
type Commit struct {
	Commit    []byte
	GroupInfo []byte
	// Welcome is nil when the commit adds nobody.
	Welcome []byte
}

// SubmitCommit accepts a commit for the group's current epoch, unless one
// was accepted first. It returns the group's new epoch.
func (service *Service) SubmitCommit(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, commit Commit) (int64, error) {
	allowed, err := service.dependencies.Members.MaySend(ctx, conversationID, me)
	if err != nil {
		return 0, err
	}
	if !allowed {
		return 0, ErrNotAllowed
	}
	group, err := service.state(ctx, conversationID)
	if err != nil {
		return 0, err
	}
	if err := checkCommit(commit, group); err != nil {
		return 0, err
	}
	lastID, err := service.dependencies.Store.Commit(ctx, store.CommitParams{
		ConversationID: conversationID, Epoch: group.Epoch, Commit: commit.Commit, GroupInfo: commit.GroupInfo,
		Welcome: commit.Welcome, SenderID: me, Now: service.dependencies.Clock.Now(),
	})
	if err != nil {
		return 0, err
	}
	return group.Epoch + 1, service.notify(ctx, conversationID, lastID)
}

// checkCommit checks the commit, its GroupInfo and its welcome, if any.
func checkCommit(commit Commit, group GroupState) error {
	if _, err := header(commit.Commit, group, mlsheader.ContentTypeCommit); err != nil {
		return err
	}
	if !mlsheader.IsWireFormat(commit.GroupInfo, mlsheader.WireFormatGroupInfo) || len(commit.GroupInfo) > MaximumMessageSize {
		return ErrInvalidMessage
	}
	if commit.Welcome != nil && (!mlsheader.IsWireFormat(commit.Welcome, mlsheader.WireFormatWelcome) || len(commit.Welcome) > MaximumMessageSize) {
		return ErrInvalidMessage
	}
	return nil
}

// Archive is the archive copy of an application message.
type Archive struct {
	MessageID uuid.UUID
	Sealed    []byte
}

// Outgoing is an application message's live copy and, when given, its archive copy.
type Outgoing struct {
	Message []byte
	Archive *Archive
	// channelID is set by SendToChannel, after checking the channel.
	channelID *uuid.UUID
}

// SendMessage accepts an application message for the current epoch, with
// its archive copy when given. It returns the live copy's ID. A server
// audience's messages go through SendToChannel instead.
func (service *Service) SendMessage(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, outgoing Outgoing) (int64, error) {
	outgoing.channelID = nil
	return service.send(ctx, me, conversationID, outgoing)
}

// send stores an application message; outgoing.channelID names its server
// channel when the conversation is that channel's audience.
func (service *Service) send(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, outgoing Outgoing) (int64, error) {
	message, archive, channelID := outgoing.Message, outgoing.Archive, outgoing.channelID
	allowed, err := service.dependencies.Members.MaySend(ctx, conversationID, me)
	if err != nil {
		return 0, err
	}
	if !allowed {
		return 0, ErrNotAllowed
	}
	group, err := service.state(ctx, conversationID)
	if err != nil {
		return 0, err
	}
	if (group.kind == conversations.KindAudience) != (channelID != nil) {
		return 0, ErrNotAllowed
	}
	if _, err := header(message, group, mlsheader.ContentTypeApplication); err != nil {
		return 0, err
	}
	params := store.SendParams{ConversationID: conversationID, Epoch: group.Epoch, Message: message, SenderID: me, Now: service.dependencies.Clock.Now(), ChannelID: channelID}
	if archive != nil {
		if len(archive.Sealed) == 0 || len(archive.Sealed) > MaximumMessageSize {
			return 0, ErrInvalidMessage
		}
		params.ArchiveID = &archive.MessageID
		params.Archive = archive.Sealed
	}
	id, err := service.dependencies.Store.Send(ctx, params)
	if err != nil {
		return 0, err
	}
	return id, service.notify(ctx, conversationID, id)
}
