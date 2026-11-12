package admin

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// UploadLimits reads and changes the instance's upload limits;
// *instanceconfig.Service implements it.
type UploadLimits interface {
	AttachmentMaxBytes(ctx context.Context) (int64, error)
	SetAttachmentMaxBytes(ctx context.Context, limit int64) error
}

// GetUploadSettings answers GET /api/v1/admin/uploads.
func (handler *Handler) GetUploadSettings(ctx context.Context, _ api.GetUploadSettingsRequestObject) (api.GetUploadSettingsResponseObject, error) {
	_, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.GetUploadSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	limit, err := handler.uploads.AttachmentMaxBytes(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetUploadSettings200JSONResponse{AttachmentMaxBytes: limit}, nil
}

// UpdateUploadSettings answers PUT /api/v1/admin/uploads.
func (handler *Handler) UpdateUploadSettings(ctx context.Context, request api.UpdateUploadSettingsRequestObject) (api.UpdateUploadSettingsResponseObject, error) {
	_, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.UpdateUploadSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	err = handler.uploads.SetAttachmentMaxBytes(ctx, request.Body.AttachmentMaxBytes)
	if errors.Is(err, instanceconfig.ErrInvalidAttachmentLimit) {
		var problems validation.Errors
		problems.Add("attachmentMaxBytes", err)
		invalid, _ := errors.AsType[*validation.Error](problems.Err())
		return api.UpdateUploadSettings422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(invalid))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateUploadSettings200JSONResponse{AttachmentMaxBytes: request.Body.AttachmentMaxBytes}, nil
}
