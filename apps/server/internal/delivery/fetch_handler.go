package delivery

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

func valueOr[T any](pointer *T, fallback T) T {
	if pointer == nil {
		return fallback
	}
	return *pointer
}

// ListMlsMessages answers GET /api/v1/conversations/{conversationId}/mls/messages.
func (handler *Handler) ListMlsMessages(ctx context.Context, request api.ListMlsMessagesRequestObject) (api.ListMlsMessagesResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	page := MessagePage{AfterID: valueOr(request.Params.After, 0), Limit: valueOr(request.Params.Limit, MaximumPage)}
	messages, err := handler.service.Messages(ctx, me, request.ConversationId, page)
	if errors.Is(err, ErrNotFound) {
		return api.ListMlsMessages404JSONResponse(notFound(err)), nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListMlsMessages200JSONResponse, 0, len(messages))
	for _, message := range messages {
		out = append(out, api.MlsMessage{
			Id: message.ID, Kind: api.MlsMessageKind(message.Kind), Epoch: message.Epoch, SenderId: message.SenderID,
			Data: message.Data, CreatedAt: message.CreatedAt, ChannelId: message.ChannelID, ThreadRootId: message.ThreadRootID,
		})
	}
	return out, nil
}

// ListArchivedMessages answers GET /api/v1/conversations/{conversationId}/archive.
func (handler *Handler) ListArchivedMessages(ctx context.Context, request api.ListArchivedMessagesRequestObject) (api.ListArchivedMessagesResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	page := ArchivePage{Before: valueOr(request.Params.Before, newest), Limit: valueOr(request.Params.Limit, MaximumPage)}
	archived, err := handler.service.ArchivedMessages(ctx, me, request.ConversationId, page)
	if errors.Is(err, ErrNotFound) {
		return api.ListArchivedMessages404JSONResponse(notFound(err)), nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListArchivedMessages200JSONResponse, 0, len(archived))
	for _, copy := range archived {
		out = append(out, api.ArchivedMessage{MessageId: copy.MessageID, Epoch: copy.Epoch, SenderId: copy.SenderID, Sealed: copy.Sealed, CreatedAt: copy.CreatedAt})
	}
	return out, nil
}

// GetArchivedMessage answers GET /api/v1/conversations/{conversationId}/archive/{messageId}.
func (handler *Handler) GetArchivedMessage(ctx context.Context, request api.GetArchivedMessageRequestObject) (api.GetArchivedMessageResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	found, err := handler.service.ConversationCopy(ctx, me, request.ConversationId, request.MessageId)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoMessage) {
		return api.GetArchivedMessage404JSONResponse(notFound(err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetArchivedMessage200JSONResponse{MessageId: found.MessageID, Epoch: found.Epoch, SenderId: found.SenderID, Sealed: found.Sealed, CreatedAt: found.CreatedAt}, nil
}
