package delivery

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// threadSetup is a channel with Ada's message in it, which threads can start on.
type threadSetup struct {
	f        fixture
	ada, sam uuid.UUID
	group    []byte
	channel  uuid.UUID
	root     uuid.UUID
}

func newThreadSetup(t *testing.T) threadSetup {
	t.Helper()
	f := newFixture(t)
	ada, sam := f.user(t, "ada"), f.user(t, "sam")
	audienceID, channelID, group := f.audience(t, ada, ada, sam)
	f.grants.grant = ChannelGrant{AudienceID: audienceID, MaySend: true, MayReplyInThreads: true, MayStartThreads: true}
	f.grants.channels = []uuid.UUID{channelID}
	setup := threadSetup{f: f, ada: ada, sam: sam, group: group, channel: channelID, root: uuid.New()}
	_, err := f.service.SendToChannel(t.Context(), setup.ref(ada), setup.outgoing(setup.root))
	require.NoError(t, err)
	return setup
}

func (setup threadSetup) ref(user uuid.UUID) ChannelRef {
	return ChannelRef{ServerID: uuid.New(), ChannelID: setup.channel, UserID: user}
}

func (setup threadSetup) outgoing(messageID uuid.UUID) Outgoing {
	return Outgoing{Message: privateMessage(setup.group, 0, 1), Archive: &Archive{MessageID: messageID, Sealed: []byte("sealed")}}
}

func (setup threadSetup) reply(t *testing.T, user uuid.UUID) (uuid.UUID, error) {
	t.Helper()
	id := uuid.New()
	_, err := setup.f.service.SendInThread(t.Context(), setup.ref(user), setup.root, setup.outgoing(id))
	return id, err
}

func (setup threadSetup) threads(t *testing.T, user uuid.UUID) []Thread {
	t.Helper()
	threads, err := setup.f.service.Threads(t.Context(), setup.ref(user))
	require.NoError(t, err)
	return threads
}

func TestThreadRepliesAreFiledUnderTheirRoot(t *testing.T) {
	setup := newThreadSetup(t)
	f := setup.f
	reply, err := setup.reply(t, setup.sam)
	require.NoError(t, err)

	messages, err := f.service.Messages(t.Context(), setup.ada, f.grants.grant.AudienceID, MessagePage{})
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Nil(t, messages[0].ThreadRootID)
	assert.Equal(t, &setup.root, messages[1].ThreadRootID)

	// The channel's archive leaves the reply out; the thread's has only it.
	later := ArchivePage{Before: now.Add(time.Hour)}
	channel, err := f.service.ChannelArchive(t.Context(), setup.ref(setup.ada), later)
	require.NoError(t, err)
	require.Len(t, channel, 1)
	assert.Equal(t, setup.root, channel[0].MessageID)
	thread, err := f.service.ThreadArchive(t.Context(), setup.ref(setup.ada), setup.root, later)
	require.NoError(t, err)
	require.Len(t, thread, 1)
	assert.Equal(t, reply, thread[0].MessageID)
	f.grants.grant.HistorySince = now.Add(time.Minute)
	thread, err = f.service.ThreadArchive(t.Context(), setup.ref(setup.ada), setup.root, later)
	require.NoError(t, err)
	assert.Empty(t, thread, "nothing from before the member may read")

	// Unread counts are the channel's own messages, not thread replies.
	states, err := f.service.ChannelReadStates(t.Context(), uuid.New(), setup.ada)
	require.NoError(t, err)
	assert.Equal(t, int64(0), states[0].UnreadCount)
}

func TestThreadsCountRepliesAndKeepFollowers(t *testing.T) {
	setup := newThreadSetup(t)
	_, err := setup.reply(t, setup.sam)
	require.NoError(t, err)

	// The replier and the root's sender follow a new thread.
	threads := setup.threads(t, setup.ada)
	require.Len(t, threads, 1)
	assert.Equal(t, Thread{RootID: setup.root, ReplyCount: 1, CreatedAt: threads[0].CreatedAt, LastReplyAt: threads[0].LastReplyAt, Following: true}, threads[0])
	assert.True(t, setup.threads(t, setup.sam)[0].Following)

	// Someone who stops following stays out until they reply themselves.
	require.NoError(t, setup.f.service.FollowThread(t.Context(), setup.ref(setup.ada), setup.root, false))
	_, err = setup.reply(t, setup.sam)
	require.NoError(t, err)
	assert.False(t, setup.threads(t, setup.ada)[0].Following)
	assert.Equal(t, int32(2), setup.threads(t, setup.ada)[0].ReplyCount)
	_, err = setup.reply(t, setup.ada)
	require.NoError(t, err)
	assert.True(t, setup.threads(t, setup.ada)[0].Following)

	require.NoError(t, setup.f.service.FollowThread(t.Context(), setup.ref(setup.ada), setup.root, false))
	require.NoError(t, setup.f.service.FollowThread(t.Context(), setup.ref(setup.ada), setup.root, true))
	assert.True(t, setup.threads(t, setup.ada)[0].Following)
}

func TestThreadsFollowTheirPermissions(t *testing.T) {
	setup := newThreadSetup(t)
	f := setup.f

	f.grants.grant.MayStartThreads = false
	_, err := setup.reply(t, setup.sam)
	require.ErrorIs(t, err, ErrNotAllowed, "starting a thread needs CREATE_THREADS")
	f.grants.grant.MayStartThreads = true
	_, err = setup.reply(t, setup.sam)
	require.NoError(t, err)
	f.grants.grant.MayStartThreads, f.grants.grant.MaySend = false, false
	_, err = setup.reply(t, setup.sam)
	require.NoError(t, err, "replying needs only SEND_IN_THREADS")
	f.grants.grant.MayReplyInThreads = false
	_, err = setup.reply(t, setup.sam)
	require.ErrorIs(t, err, ErrNotAllowed)
}

func TestThreadsStartOnlyOnChannelMessages(t *testing.T) {
	setup := newThreadSetup(t)
	f := setup.f
	reply, err := setup.reply(t, setup.sam)
	require.NoError(t, err)

	for _, root := range []uuid.UUID{uuid.New(), reply} {
		_, err := f.service.SendInThread(t.Context(), setup.ref(setup.sam), root, setup.outgoing(uuid.New()))
		require.ErrorIs(t, err, ErrNoThread)
	}
	_, err = f.service.SendInThread(t.Context(), setup.ref(setup.sam), setup.root, Outgoing{Message: privateMessage(setup.group, 0, 1)})
	require.ErrorIs(t, err, ErrInvalidMessage, "a reply needs its archive copy")
	require.ErrorIs(t, f.service.FollowThread(t.Context(), setup.ref(setup.sam), uuid.New(), true), ErrNoThread)

	f.grants.err = ErrNotFound
	_, err = setup.reply(t, setup.sam)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.Threads(t.Context(), setup.ref(setup.sam))
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.ThreadArchive(t.Context(), setup.ref(setup.sam), setup.root, ArchivePage{})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, f.service.FollowThread(t.Context(), setup.ref(setup.sam), setup.root, true), ErrNotFound)

	f.grants.err = nil
	f.pool.Close()
	_, err = setup.reply(t, setup.sam)
	require.Error(t, err)
	_, err = f.service.Threads(t.Context(), setup.ref(setup.sam))
	require.Error(t, err)
	_, err = f.service.ThreadArchive(t.Context(), setup.ref(setup.sam), setup.root, ArchivePage{})
	require.Error(t, err)
	require.Error(t, f.service.FollowThread(t.Context(), setup.ref(setup.sam), setup.root, true))
}

func TestThreadEndpoints(t *testing.T) {
	setup := newThreadSetup(t)
	f := setup.f
	handler := NewHandler(f.service)
	ctx := as(t, setup.sam)
	send := func(root *uuid.UUID, archive bool) string {
		body := api.ChannelSend{Message: privateMessage(setup.group, 0, 1), ThreadRootId: root}
		if archive {
			body.Archive = &api.ArchiveCopy{MessageId: uuid.New(), Sealed: []byte("sealed")}
		}
		response, err := handler.SendChannelMessage(ctx, api.SendChannelMessageRequestObject{ChannelId: setup.channel, Body: &body})
		require.NoError(t, err)
		return typeName(response)
	}
	missing := uuid.New()

	assert.Equal(t, "SendChannelMessage201JSONResponse", send(&setup.root, true))
	assert.Equal(t, "SendChannelMessage404JSONResponse", send(&missing, true))
	assert.Equal(t, "SendChannelMessage422JSONResponse", send(&setup.root, false))

	listed := must(handler.ListChannelThreads(ctx, api.ListChannelThreadsRequestObject{ChannelId: setup.channel}))
	threads, ok := listed.(api.ListChannelThreads200JSONResponse)
	require.True(t, ok)
	require.Len(t, threads, 1)
	assert.Equal(t, 1, threads[0].ReplyCount)
	archive := must(handler.ListThreadArchive(ctx, api.ListThreadArchiveRequestObject{ChannelId: setup.channel, RootId: setup.root}))
	copies, ok := archive.(api.ListThreadArchive200JSONResponse)
	require.True(t, ok)
	require.Len(t, copies, 1)
	assert.Equal(t, &setup.root, copies[0].ThreadRootId)

	assert.Equal(t, "UnfollowThread204Response", typeName(must(handler.UnfollowThread(ctx, api.UnfollowThreadRequestObject{ChannelId: setup.channel, RootId: setup.root}))))
	assert.Equal(t, "FollowThread204Response", typeName(must(handler.FollowThread(ctx, api.FollowThreadRequestObject{ChannelId: setup.channel, RootId: setup.root}))))
	assert.Equal(t, "FollowThread404JSONResponse", typeName(must(handler.FollowThread(ctx, api.FollowThreadRequestObject{ChannelId: setup.channel, RootId: missing}))))

	f.grants.err = ErrNotFound
	for _, response := range []any{
		must(handler.ListChannelThreads(ctx, api.ListChannelThreadsRequestObject{ChannelId: setup.channel})),
		must(handler.ListThreadArchive(ctx, api.ListThreadArchiveRequestObject{ChannelId: setup.channel, RootId: setup.root})),
		must(handler.FollowThread(ctx, api.FollowThreadRequestObject{ChannelId: setup.channel, RootId: setup.root})),
		must(handler.UnfollowThread(ctx, api.UnfollowThreadRequestObject{ChannelId: setup.channel, RootId: setup.root})),
	} {
		assert.Contains(t, typeName(response), "404")
	}

	f.grants.err = errors.New("database down")
	for _, call := range []func() error{
		func() error {
			_, err := handler.ListChannelThreads(ctx, api.ListChannelThreadsRequestObject{})
			return err
		},
		func() error {
			_, err := handler.ListThreadArchive(ctx, api.ListThreadArchiveRequestObject{})
			return err
		},
		func() error { _, err := handler.FollowThread(ctx, api.FollowThreadRequestObject{}); return err },
		func() error { _, err := handler.UnfollowThread(ctx, api.UnfollowThreadRequestObject{}); return err },
	} {
		require.Error(t, call())
	}
}
