package servers

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
	path := "/api/v1/server-icons/" + iconID.String()
	return &path
}

// SetServerIcon answers PUT /api/v1/servers/{serverId}/icon.
func (handler *Handler) SetServerIcon(ctx context.Context, request api.SetServerIconRequestObject) (api.SetServerIconResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	// One byte more than allowed tells a too-large file from one exactly
	// at the limit without reading the rest.
	upload, err := io.ReadAll(io.LimitReader(request.Body, avatar.MaximumUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading icon: %w", err)
	}
	err = handler.service.SetIcon(ctx, where, upload)
	switch {
	case errors.Is(err, ErrForbidden):
		return api.SetServerIcon403JSONResponse(apiresponse.Error("missing_permission", err)), nil
	case errors.Is(err, ErrNotFound):
		return api.SetServerIcon404JSONResponse(apiresponse.Error("server_not_found", err)), nil
	case errors.Is(err, avatar.ErrTooLarge):
		return api.SetServerIcon413JSONResponse(apiresponse.Error("image_too_large", err)), nil
	case errors.Is(err, avatar.ErrUnsupported), errors.Is(err, avatar.ErrDamaged):
		return api.SetServerIcon422JSONResponse(apiresponse.Error("unsupported_image", err)), nil
	case errors.Is(err, ErrRateLimited):
		return api.SetServerIcon429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case err != nil:
		return nil, err
	}
	return api.SetServerIcon204Response{}, nil
}

// RemoveServerIcon answers DELETE /api/v1/servers/{serverId}/icon.
func (handler *Handler) RemoveServerIcon(ctx context.Context, request api.RemoveServerIconRequestObject) (api.RemoveServerIconResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.RemoveIcon(ctx, where)
	switch {
	case errors.Is(err, ErrForbidden):
		return api.RemoveServerIcon403JSONResponse(apiresponse.Error("missing_permission", err)), nil
	case errors.Is(err, ErrNotFound):
		return api.RemoveServerIcon404JSONResponse(apiresponse.Error("server_not_found", err)), nil
	case err != nil:
		return nil, err
	}
	return api.RemoveServerIcon204Response{}, nil
}

// GetServerIcon answers GET /api/v1/server-icons/{iconId}. The generated
// code closes the file after copying it to the response.
func (handler *Handler) GetServerIcon(ctx context.Context, request api.GetServerIconRequestObject) (api.GetServerIconResponseObject, error) {
	icon, err := handler.service.OpenIcon(ctx, request.IconId)
	if errors.Is(err, ErrIconNotFound) {
		return api.GetServerIcon404JSONResponse(apiresponse.Error("icon_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	headers := api.GetServerIcon200ResponseHeaders{CacheControl: iconCacheControl}
	switch icon.ContentType {
	case "image/gif":
		return api.GetServerIcon200ImagegifResponse{Body: icon.Content, Headers: headers}, nil
	case "image/jpeg":
		return api.GetServerIcon200ImagejpegResponse{Body: icon.Content, Headers: headers}, nil
	default:
		return api.GetServerIcon200ImagepngResponse{Body: icon.Content, Headers: headers}, nil
	}
}
