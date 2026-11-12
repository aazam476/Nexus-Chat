package admin

import (
	"context"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// PreviewProxy reads and changes the link preview proxy setting;
// *instanceconfig.Service implements it.
type PreviewProxy interface {
	LinkPreviewProxy(ctx context.Context) (bool, error)
	SetLinkPreviewProxy(ctx context.Context, enabled bool) error
}

// GetPreviewSettings answers GET /api/v1/admin/previews.
func (handler *Handler) GetPreviewSettings(ctx context.Context, _ api.GetPreviewSettingsRequestObject) (api.GetPreviewSettingsResponseObject, error) {
	_, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.GetPreviewSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	enabled, err := handler.previews.LinkPreviewProxy(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetPreviewSettings200JSONResponse{ProxyEnabled: enabled}, nil
}

// UpdatePreviewSettings answers PUT /api/v1/admin/previews.
func (handler *Handler) UpdatePreviewSettings(ctx context.Context, request api.UpdatePreviewSettingsRequestObject) (api.UpdatePreviewSettingsResponseObject, error) {
	_, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.UpdatePreviewSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := handler.previews.SetLinkPreviewProxy(ctx, request.Body.ProxyEnabled); err != nil {
		return nil, err
	}
	return api.UpdatePreviewSettings200JSONResponse{ProxyEnabled: request.Body.ProxyEnabled}, nil
}
