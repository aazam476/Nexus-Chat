package delivery

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

func channelNotFound(err error) api.Error {
	return apiresponse.Error("channel_not_found", err)
}

// SendChannelMessage answers POST /api/v1/servers/{serverId}/channels/{channelId}/messages.
func (handler *Handler) SendChannelMessage(ctx context.Context, request api.SendChannelMessageRequestObject) (api.SendChannelMessageResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	outgoing := Outgoing{Message: request.Body.Message}
	if request.Body.Archive != nil {
		outgoing.Archive = &Archive{MessageID: request.Body.Archive.MessageId, Sealed: request.Body.Archive.Sealed}
	}
	ref := ChannelRef{ServerID: request.ServerId, ChannelID: request.ChannelId, UserID: me}
	id, err := handler.service.SendToChannel(ctx, ref, outgoing)
	switch {
	case errors.Is(err, ErrNotAllowed):
		return api.SendChannelMessage403JSONResponse(apiresponse.Error("cannot_send", err)), nil
	case errors.Is(err, ErrNotFound):
		return api.SendChannelMessage404JSONResponse(channelNotFound(err)), nil
	case errors.Is(err, ErrWrongEpoch):
		return api.SendChannelMessage409JSONResponse(apiresponse.Error("wrong_epoch", err)), nil
	case errors.Is(err, ErrDuplicate):
		return api.SendChannelMessage409JSONResponse(apiresponse.Error("duplicate_message", err)), nil
	case errors.Is(err, ErrInvalidMessage):
		return api.SendChannelMessage422JSONResponse(apiresponse.Error("invalid_mls_message", err)), nil
	case err != nil:
		return nil, err
	}
	return api.SendChannelMessage201JSONResponse{Id: id}, nil
}

// ListChannelArchive answers GET /api/v1/servers/{serverId}/channels/{channelId}/archive.
func (handler *Handler) ListChannelArchive(ctx context.Context, request api.ListChannelArchiveRequestObject) (api.ListChannelArchiveResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	ref := ChannelRef{ServerID: request.ServerId, ChannelID: request.ChannelId, UserID: me}
	page := ArchivePage{Before: valueOr(request.Params.Before, newest), Limit: valueOr(request.Params.Limit, MaximumPage)}
	archived, err := handler.service.ChannelArchive(ctx, ref, page)
	if errors.Is(err, ErrNotFound) {
		return api.ListChannelArchive404JSONResponse(channelNotFound(err)), nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListChannelArchive200JSONResponse, 0, len(archived))
	for _, copy := range archived {
		out = append(out, api.ChannelArchivedMessage{
			MessageId: copy.MessageID, MlsGroupId: copy.MLSGroupID, Epoch: copy.Epoch, SenderId: copy.SenderID,
			Sealed: copy.Sealed, CreatedAt: copy.CreatedAt,
		})
	}
	return out, nil
}
