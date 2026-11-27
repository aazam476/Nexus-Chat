package delivery

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

func threadNotFound(err error) api.Error {
	if errors.Is(err, ErrNoThread) {
		return apiresponse.Error("thread_not_found", err)
	}
	return channelNotFound(err)
}

// ListChannelThreads answers GET /api/v1/servers/{serverId}/channels/{channelId}/threads.
func (handler *Handler) ListChannelThreads(ctx context.Context, request api.ListChannelThreadsRequestObject) (api.ListChannelThreadsResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	threads, err := handler.service.Threads(ctx, ChannelRef{ServerID: request.ServerId, ChannelID: request.ChannelId, UserID: me})
	if errors.Is(err, ErrNotFound) {
		return api.ListChannelThreads404JSONResponse(channelNotFound(err)), nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListChannelThreads200JSONResponse, 0, len(threads))
	for _, thread := range threads {
		out = append(out, api.ChannelThread{
			RootId: thread.RootID, ReplyCount: int(thread.ReplyCount), CreatedAt: thread.CreatedAt, LastReplyAt: thread.LastReplyAt, Following: thread.Following,
		})
	}
	return out, nil
}

// ListThreadArchive answers GET /api/v1/servers/{serverId}/channels/{channelId}/threads/{rootId}/archive.
func (handler *Handler) ListThreadArchive(ctx context.Context, request api.ListThreadArchiveRequestObject) (api.ListThreadArchiveResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	ref := ChannelRef{ServerID: request.ServerId, ChannelID: request.ChannelId, UserID: me}
	page := ArchivePage{Before: valueOr(request.Params.Before, newest), Limit: valueOr(request.Params.Limit, MaximumPage)}
	archived, err := handler.service.ThreadArchive(ctx, ref, request.RootId, page)
	if errors.Is(err, ErrNotFound) {
		return api.ListThreadArchive404JSONResponse(channelNotFound(err)), nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListThreadArchive200JSONResponse, 0, len(archived))
	for _, copy := range archived {
		out = append(out, api.ChannelArchivedMessage{
			MessageId: copy.MessageID, MlsGroupId: copy.MLSGroupID, Epoch: copy.Epoch, SenderId: copy.SenderID,
			Sealed: copy.Sealed, CreatedAt: copy.CreatedAt, ThreadRootId: &request.RootId,
		})
	}
	return out, nil
}

// FollowThread answers PUT /api/v1/servers/{serverId}/channels/{channelId}/threads/{rootId}/follow.
func (handler *Handler) FollowThread(ctx context.Context, request api.FollowThreadRequestObject) (api.FollowThreadResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.FollowThread(ctx, ChannelRef{ServerID: request.ServerId, ChannelID: request.ChannelId, UserID: me}, request.RootId, true)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoThread) {
		return api.FollowThread404JSONResponse(threadNotFound(err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.FollowThread204Response{}, nil
}

// UnfollowThread answers DELETE /api/v1/servers/{serverId}/channels/{channelId}/threads/{rootId}/follow.
func (handler *Handler) UnfollowThread(ctx context.Context, request api.UnfollowThreadRequestObject) (api.UnfollowThreadResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.FollowThread(ctx, ChannelRef{ServerID: request.ServerId, ChannelID: request.ChannelId, UserID: me}, request.RootId, false)
	if errors.Is(err, ErrNotFound) {
		return api.UnfollowThread404JSONResponse(channelNotFound(err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.UnfollowThread204Response{}, nil
}
