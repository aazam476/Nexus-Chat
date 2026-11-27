package delivery

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
)

// Threads (spec §4.5): replies on a channel message, in the channel's
// audience group. The server checks the root is a message of the channel
// that is not itself a reply, counts replies, and keeps who follows each
// thread; it never sees what the replies say.

// ErrNoThread means there is no such thread, or no message in the channel
// a thread can start on.
var ErrNoThread = errors.New("there is no such thread in this channel")

// Thread is one of a channel's threads as a member sees it.
type Thread struct {
	RootID      uuid.UUID
	ReplyCount  int32
	CreatedAt   time.Time
	LastReplyAt time.Time
	Following   bool
}

// SendInThread accepts a reply in the thread on rootID, starting the
// thread with its first reply. A reply must have an archive copy, so the
// thread can be read later.
func (service *Service) SendInThread(ctx context.Context, ref ChannelRef, rootID uuid.UUID, outgoing Outgoing) (int64, error) {
	grant, err := service.dependencies.Channels.ChannelGrant(ctx, ref)
	if err != nil {
		return 0, err
	}
	if outgoing.Archive == nil {
		return 0, ErrInvalidMessage
	}
	rootSender, err := service.dependencies.Store.ThreadRoot(ctx, store.ThreadRootParams{ChannelID: &ref.ChannelID, MessageID: rootID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoThread
	}
	if err != nil {
		return 0, err
	}
	exists, err := service.dependencies.Store.ThreadExists(ctx, store.ThreadExistsParams{ChannelID: ref.ChannelID, RootMessageID: rootID})
	if err != nil {
		return 0, err
	}
	if !grant.MayReplyInThreads || (!exists && !grant.MayStartThreads) {
		return 0, ErrNotAllowed
	}
	reply := &store.ThreadReply{RootID: rootID}
	if !exists {
		// The root's sender follows a thread on their message once, when it starts.
		reply.RootSender = rootSender
	}
	outgoing.channelID, outgoing.thread = &ref.ChannelID, reply
	return service.send(ctx, ref.UserID, grant.AudienceID, outgoing)
}

// Threads lists the channel's threads, most recently active first.
func (service *Service) Threads(ctx context.Context, ref ChannelRef) ([]Thread, error) {
	if _, err := service.dependencies.Channels.ChannelGrant(ctx, ref); err != nil {
		return nil, err
	}
	rows, err := service.dependencies.Store.ChannelThreads(ctx, store.ChannelThreadsParams{UserID: ref.UserID, ChannelID: ref.ChannelID})
	if err != nil {
		return nil, err
	}
	threads := make([]Thread, 0, len(rows))
	for _, row := range rows {
		threads = append(threads, Thread{RootID: row.RootMessageID, ReplyCount: row.ReplyCount, CreatedAt: row.CreatedAt, LastReplyAt: row.LastReplyAt, Following: row.Following})
	}
	return threads, nil
}

// ThreadArchive returns the archive copies of a thread's replies made
// before page.Before, newest first, back to where the user may read.
func (service *Service) ThreadArchive(ctx context.Context, ref ChannelRef, rootID uuid.UUID, page ArchivePage) ([]ChannelArchived, error) {
	grant, err := service.dependencies.Channels.ChannelGrant(ctx, ref)
	if err != nil {
		return nil, err
	}
	rows, err := service.dependencies.Store.ThreadArchiveBefore(ctx, store.ThreadArchiveBeforeParams{
		ChannelID: &ref.ChannelID, RootMessageID: &rootID, Before: page.Before, Since: grant.HistorySince, RowLimit: pageSize(page.Limit),
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

// FollowThread has the user follow a thread, or stop following it.
func (service *Service) FollowThread(ctx context.Context, ref ChannelRef, rootID uuid.UUID, follow bool) error {
	if _, err := service.dependencies.Channels.ChannelGrant(ctx, ref); err != nil {
		return err
	}
	if !follow {
		return service.dependencies.Store.UnfollowThread(ctx, store.UnfollowThreadParams{ChannelID: ref.ChannelID, RootMessageID: rootID, UserID: ref.UserID})
	}
	exists, err := service.dependencies.Store.ThreadExists(ctx, store.ThreadExistsParams{ChannelID: ref.ChannelID, RootMessageID: rootID})
	if err != nil {
		return err
	}
	if !exists {
		return ErrNoThread
	}
	_, err = service.dependencies.Store.FollowThread(ctx, store.FollowThreadParams{UserID: ref.UserID, ChannelID: ref.ChannelID, RootMessageID: rootID})
	return err
}
