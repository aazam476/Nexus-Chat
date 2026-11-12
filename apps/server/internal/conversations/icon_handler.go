package conversations

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
)

// iconCacheControl lets browsers keep an icon forever: a new icon always
// has a new address.
const iconCacheControl = "public, max-age=31536000, immutable"

func iconPath(iconID *uuid.UUID) *string {
	if iconID == nil {
		return nil
	}
	path := "/api/v1/conversation-icons/" + iconID.String()
	return &path
}

// SetConversationIcon answers PUT /api/v1/conversations/{conversationId}/icon.
func (handler *Handler) SetConversationIcon(ctx context.Context, request api.SetConversationIconRequestObject) (api.SetConversationIconResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	// One byte more than allowed tells a too-large file from one exactly
	// at the limit without reading the rest.
	upload, err := io.ReadAll(io.LimitReader(request.Body, avatar.MaximumUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading icon: %w", err)
	}
	conversation, err := handler.service.SetIcon(ctx, me, request.ConversationId, upload)
	code, _ := errorCode(err)
	switch {
	case errors.Is(err, ErrNotFound):
		return api.SetConversationIcon404JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrNotAGroup):
		return api.SetConversationIcon422JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, avatar.ErrTooLarge):
		return api.SetConversationIcon413JSONResponse(apiresponse.Error("image_too_large", err)), nil
	case errors.Is(err, avatar.ErrUnsupported), errors.Is(err, avatar.ErrDamaged):
		return api.SetConversationIcon422JSONResponse(apiresponse.Error("unsupported_image", err)), nil
	case errors.Is(err, ErrRateLimited):
		return api.SetConversationIcon429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case err != nil:
		return nil, err
	}
	return api.SetConversationIcon200JSONResponse(response(conversation)), nil
}

// RemoveConversationIcon answers DELETE /api/v1/conversations/{conversationId}/icon.
func (handler *Handler) RemoveConversationIcon(ctx context.Context, request api.RemoveConversationIconRequestObject) (api.RemoveConversationIconResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	conversation, err := handler.service.RemoveIcon(ctx, me, request.ConversationId)
	code, _ := errorCode(err)
	switch {
	case errors.Is(err, ErrNotFound):
		return api.RemoveConversationIcon404JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrNotAGroup):
		return api.RemoveConversationIcon422JSONResponse(apiresponse.Error(code, err)), nil
	case err != nil:
		return nil, err
	}
	return api.RemoveConversationIcon200JSONResponse(response(conversation)), nil
}

// GetConversationIcon answers GET /api/v1/conversation-icons/{iconId}. The
// generated code closes the file after copying it to the response.
func (handler *Handler) GetConversationIcon(ctx context.Context, request api.GetConversationIconRequestObject) (api.GetConversationIconResponseObject, error) {
	icon, err := handler.service.OpenIcon(ctx, request.IconId)
	if errors.Is(err, ErrIconNotFound) {
		return api.GetConversationIcon404JSONResponse(apiresponse.Error("icon_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	headers := api.GetConversationIcon200ResponseHeaders{CacheControl: iconCacheControl}
	switch icon.ContentType {
	case "image/gif":
		return api.GetConversationIcon200ImagegifResponse{Body: icon.Content, Headers: headers}, nil
	case "image/jpeg":
		return api.GetConversationIcon200ImagejpegResponse{Body: icon.Content, Headers: headers}, nil
	default:
		return api.GetConversationIcon200ImagepngResponse{Body: icon.Content, Headers: headers}, nil
	}
}
