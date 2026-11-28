package delivery

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
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
	var id int64
	if request.Body.ThreadRootId == nil {
		id, err = handler.service.SendToChannel(ctx, ref, outgoing)
	} else {
		id, err = handler.service.SendInThread(ctx, ref, *request.Body.ThreadRootId, outgoing)
	}
	if err != nil {
		return sendChannelError(err)
	}
	return api.SendChannelMessage201JSONResponse{Id: id}, nil
}

func sendChannelError(err error) (api.SendChannelMessageResponseObject, error) {
	switch {
	case errors.Is(err, ErrNotAllowed):
		return api.SendChannelMessage403JSONResponse(apiresponse.Error("cannot_send", err)), nil
	case errors.Is(err, ErrNotFound):
		return api.SendChannelMessage404JSONResponse(channelNotFound(err)), nil
	case errors.Is(err, ErrNoThread):
		return api.SendChannelMessage404JSONResponse(apiresponse.Error("thread_not_found", err)), nil
	case errors.Is(err, ErrWrongEpoch):
		return api.SendChannelMessage409JSONResponse(apiresponse.Error("wrong_epoch", err)), nil
	case errors.Is(err, ErrDuplicate):
		return api.SendChannelMessage409JSONResponse(apiresponse.Error("duplicate_message", err)), nil
	case errors.Is(err, ErrInvalidMessage):
		return api.SendChannelMessage422JSONResponse(apiresponse.Error("invalid_mls_message", err)), nil
	}
	return nil, err
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

// MarkChannelRead answers PUT /api/v1/servers/{serverId}/channels/{channelId}/read.
func (handler *Handler) MarkChannelRead(ctx context.Context, request api.MarkChannelReadRequestObject) (api.MarkChannelReadResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	ref := ChannelRef{ServerID: request.ServerId, ChannelID: request.ChannelId, UserID: me}
	position, err := handler.service.MarkChannelRead(ctx, ref, request.Body.LastReadId)
	if errors.Is(err, ErrNotFound) {
		return api.MarkChannelRead404JSONResponse(channelNotFound(err)), nil
	}
	if problems, ok := errors.AsType[*validation.Error](err); ok {
		return api.MarkChannelRead422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	}
	if err != nil {
		return nil, err
	}
	state := api.ChannelReadState{ChannelId: request.ChannelId, LastReadId: position}
	states, err := handler.service.ChannelReadStates(ctx, request.ServerId, me)
	if err != nil {
		return nil, err
	}
	for _, each := range states {
		if each.ChannelID == request.ChannelId {
			state.UnreadCount = each.UnreadCount
		}
	}
	return api.MarkChannelRead200JSONResponse(state), nil
}

// ListChannelReadStates answers GET /api/v1/servers/{serverId}/read-state.
func (handler *Handler) ListChannelReadStates(ctx context.Context, request api.ListChannelReadStatesRequestObject) (api.ListChannelReadStatesResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	states, err := handler.service.ChannelReadStates(ctx, request.ServerId, me)
	if errors.Is(err, ErrNotFound) {
		return api.ListChannelReadStates404JSONResponse(apiresponse.Error("server_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListChannelReadStates200JSONResponse, 0, len(states))
	for _, state := range states {
		out = append(out, api.ChannelReadState{ChannelId: state.ChannelID, LastReadId: state.LastReadID, UnreadCount: state.UnreadCount})
	}
	return out, nil
}

// DeleteChannelMessage answers DELETE /api/v1/servers/{serverId}/channels/{channelId}/messages/{messageId}.
func (handler *Handler) DeleteChannelMessage(ctx context.Context, request api.DeleteChannelMessageRequestObject) (api.DeleteChannelMessageResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.DeleteChannelMessage(ctx, ChannelRef{ServerID: request.ServerId, ChannelID: request.ChannelId, UserID: me}, request.MessageId)
	switch {
	case errors.Is(err, ErrNotAllowed):
		return api.DeleteChannelMessage403JSONResponse(apiresponse.Error("missing_permission", err)), nil
	case errors.Is(err, ErrNotFound):
		return api.DeleteChannelMessage404JSONResponse(channelNotFound(err)), nil
	case errors.Is(err, ErrNoMessage):
		return api.DeleteChannelMessage404JSONResponse(apiresponse.Error("message_not_found", err)), nil
	case err != nil:
		return nil, err
	}
	return api.DeleteChannelMessage204Response{}, nil
}
