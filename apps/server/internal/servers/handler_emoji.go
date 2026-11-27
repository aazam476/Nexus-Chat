package servers

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
)

// ListServerEmoji answers GET /api/v1/servers/{serverId}/emoji.
func (handler *Handler) ListServerEmoji(ctx context.Context, request api.ListServerEmojiRequestObject) (api.ListServerEmojiResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	list, err := handler.service.ListEmoji(ctx, where)
	if errors.Is(err, ErrNotFound) {
		return api.ListServerEmoji404JSONResponse(apiresponse.Error("server_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	response := make(api.ListServerEmoji200JSONResponse, len(list))
	for index, emoji := range list {
		response[index] = api.ServerEmoji{
			Id: emoji.ID, Name: emoji.Name, Animated: emoji.Animated, CreatedBy: emoji.CreatedBy,
			CreatedAt: emoji.CreatedAt, Url: "/api/v1/emoji/" + emoji.ID.String(),
		}
	}
	return response, nil
}

// AddServerEmoji answers POST /api/v1/servers/{serverId}/emoji.
func (handler *Handler) AddServerEmoji(ctx context.Context, request api.AddServerEmojiRequestObject) (api.AddServerEmojiResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	// One byte more than allowed tells a too-large file from one exactly
	// at the limit without reading the rest.
	upload, err := io.ReadAll(io.LimitReader(request.Body, EmojiMaximumBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading emoji: %w", err)
	}
	id, err := handler.service.AddEmoji(ctx, where, request.Params.Name, upload)
	if err != nil {
		return addEmojiError(err)
	}
	return api.AddServerEmoji201JSONResponse{Id: id}, nil
}

func addEmojiError(err error) (api.AddServerEmojiResponseObject, error) {
	switch {
	case errors.Is(err, ErrForbidden):
		return api.AddServerEmoji403JSONResponse(apiresponse.Error("missing_permission", err)), nil
	case errors.Is(err, ErrNotFound):
		return api.AddServerEmoji404JSONResponse(apiresponse.Error("server_not_found", err)), nil
	case errors.Is(err, ErrEmojiTaken):
		return api.AddServerEmoji409JSONResponse(apiresponse.Error("emoji_name_taken", err)), nil
	case errors.Is(err, ErrEmojiLimit):
		return api.AddServerEmoji409JSONResponse(apiresponse.Error("emoji_limit", err)), nil
	case errors.Is(err, avatar.ErrTooLarge):
		return api.AddServerEmoji413JSONResponse(apiresponse.Error("image_too_large", err)), nil
	case errors.Is(err, ErrEmojiName):
		return api.AddServerEmoji422JSONResponse(apiresponse.Error("invalid_emoji_name", err)), nil
	case errors.Is(err, avatar.ErrUnsupported), errors.Is(err, avatar.ErrDamaged), errors.Is(err, avatar.ErrNotAllowed):
		return api.AddServerEmoji422JSONResponse(apiresponse.Error("unsupported_image", err)), nil
	case errors.Is(err, ErrRateLimited):
		return api.AddServerEmoji429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	}
	return nil, err
}

// RemoveServerEmoji answers DELETE /api/v1/servers/{serverId}/emoji/{emojiId}.
func (handler *Handler) RemoveServerEmoji(ctx context.Context, request api.RemoveServerEmojiRequestObject) (api.RemoveServerEmojiResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.RemoveEmoji(ctx, where, request.EmojiId)
	switch {
	case errors.Is(err, ErrForbidden):
		return api.RemoveServerEmoji403JSONResponse(apiresponse.Error("missing_permission", err)), nil
	case errors.Is(err, ErrNotFound):
		return api.RemoveServerEmoji404JSONResponse(apiresponse.Error("server_not_found", err)), nil
	case errors.Is(err, ErrEmojiNotFound):
		return api.RemoveServerEmoji404JSONResponse(apiresponse.Error("emoji_not_found", err)), nil
	case err != nil:
		return nil, err
	}
	return api.RemoveServerEmoji204Response{}, nil
}

// GetEmoji answers GET /api/v1/emoji/{emojiId}. The generated code closes
// the file after copying it to the response.
func (handler *Handler) GetEmoji(ctx context.Context, request api.GetEmojiRequestObject) (api.GetEmojiResponseObject, error) {
	picture, err := handler.service.OpenEmoji(ctx, request.EmojiId)
	if errors.Is(err, ErrEmojiNotFound) {
		return api.GetEmoji404JSONResponse(apiresponse.Error("emoji_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	headers := api.GetEmoji200ResponseHeaders{CacheControl: iconCacheControl}
	if picture.ContentType == "image/gif" {
		return api.GetEmoji200ImagegifResponse{Body: picture.Content, Headers: headers}, nil
	}
	return api.GetEmoji200ImagepngResponse{Body: picture.Content, Headers: headers}, nil
}
