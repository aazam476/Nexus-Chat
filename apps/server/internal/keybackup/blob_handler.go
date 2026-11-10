package keybackup

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// GetBackupBlob answers GET /api/v1/key-backup/blobs/{name}.
func (handler *Handler) GetBackupBlob(ctx context.Context, request api.GetBackupBlobRequestObject) (api.GetBackupBlobResponseObject, error) {
	caller, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	data, err := handler.service.GetBlob(ctx, caller, request.Name)
	if errors.Is(err, ErrNotFound) {
		return api.GetBackupBlob404JSONResponse(apiresponse.Error("backup_item_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetBackupBlob200JSONResponse{Data: data}, nil
}

// PutBackupBlob answers PUT /api/v1/key-backup/blobs/{name}.
func (handler *Handler) PutBackupBlob(ctx context.Context, request api.PutBackupBlobRequestObject) (api.PutBackupBlobResponseObject, error) {
	caller, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.service.PutBlob(ctx, caller, request.Name, request.Body.Data)
	switch {
	case errors.Is(err, ErrInvalidBlobName):
		return api.PutBackupBlob400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(apiresponse.Error("bad_request", err))}, nil
	case errors.Is(err, ErrInvalidBlob):
		return api.PutBackupBlob422JSONResponse(apiresponse.Error("invalid_backup_blob", err)), nil
	case errors.Is(err, ErrTooManyBlobs):
		return api.PutBackupBlob409JSONResponse(apiresponse.Error("backup_full", err)), nil
	case err != nil:
		return nil, err
	}
	return api.PutBackupBlob204Response{}, nil
}

// DeleteBackupBlob answers DELETE /api/v1/key-backup/blobs/{name}.
func (handler *Handler) DeleteBackupBlob(ctx context.Context, request api.DeleteBackupBlobRequestObject) (api.DeleteBackupBlobResponseObject, error) {
	caller, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.service.DeleteBlob(ctx, caller, request.Name)
	if errors.Is(err, ErrNotFound) {
		return api.DeleteBackupBlob404JSONResponse(apiresponse.Error("backup_item_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.DeleteBackupBlob204Response{}, nil
}
