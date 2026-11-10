package keybackup

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// Handler serves the key-backup endpoints.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func reauthentication(err error) api.ReauthenticationRequiredJSONResponse {
	return api.ReauthenticationRequiredJSONResponse(apiresponse.Error("reauthentication_required", err))
}

// GetKeyBackup answers GET /api/v1/key-backup.
func (handler *Handler) GetKeyBackup(ctx context.Context, _ api.GetKeyBackupRequestObject) (api.GetKeyBackupResponseObject, error) {
	caller, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	backup, err := handler.service.Get(ctx, caller)
	if err != nil {
		return nil, err
	}
	response := api.GetKeyBackup200JSONResponse{
		Salt: backup.Salt, WrappedKeys: make([]api.WrappedBackupKey, 0, len(backup.WrappedKeys)),
		Blobs: make([]api.BackupBlobInfo, 0, len(backup.Blobs)),
	}
	for _, key := range backup.WrappedKeys {
		wrapped := api.WrappedBackupKey{Method: api.UnlockMethod(key.Method), WrappedKey: key.WrappedKey}
		if key.Parameters != nil {
			parameters := key.Parameters
			wrapped.Parameters = &parameters
		}
		response.WrappedKeys = append(response.WrappedKeys, wrapped)
	}
	for _, blob := range backup.Blobs {
		response.Blobs = append(response.Blobs, api.BackupBlobInfo{Name: blob.Name, Size: int32(blob.Size), UpdatedAt: blob.UpdatedAt}) //nolint:gosec // at most MaximumBlobSize
	}
	return response, nil
}

// PutWrappedBackupKey answers PUT /api/v1/key-backup/wrapped-keys/{method}.
func (handler *Handler) PutWrappedBackupKey(ctx context.Context, request api.PutWrappedBackupKeyRequestObject) (api.PutWrappedBackupKeyResponseObject, error) {
	caller, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	key := WrappedKey{Method: string(request.Method), WrappedKey: request.Body.WrappedKey}
	if request.Body.Parameters != nil {
		key.Parameters = *request.Body.Parameters
	}
	err := handler.service.PutWrappedKey(ctx, caller, key)
	switch {
	case errors.Is(err, ErrUnknownMethod):
		return api.PutWrappedBackupKey400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(apiresponse.Error("bad_request", err))}, nil
	case errors.Is(err, ErrInvalidWrapped):
		return api.PutWrappedBackupKey422JSONResponse(apiresponse.Error("invalid_wrapped_key", err)), nil
	case errors.Is(err, session.ErrReauthenticationRequired):
		return api.PutWrappedBackupKey403JSONResponse{ReauthenticationRequiredJSONResponse: reauthentication(err)}, nil
	case err != nil:
		return nil, err
	}
	return api.PutWrappedBackupKey204Response{}, nil
}

// DeleteWrappedBackupKey answers DELETE /api/v1/key-backup/wrapped-keys/{method}.
func (handler *Handler) DeleteWrappedBackupKey(ctx context.Context, request api.DeleteWrappedBackupKeyRequestObject) (api.DeleteWrappedBackupKeyResponseObject, error) {
	caller, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.service.DeleteWrappedKey(ctx, caller, string(request.Method))
	switch {
	case errors.Is(err, ErrUnknownMethod):
		return api.DeleteWrappedBackupKey400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(apiresponse.Error("bad_request", err))}, nil
	case errors.Is(err, session.ErrReauthenticationRequired):
		return api.DeleteWrappedBackupKey403JSONResponse{ReauthenticationRequiredJSONResponse: reauthentication(err)}, nil
	case errors.Is(err, ErrNotFound):
		return api.DeleteWrappedBackupKey404JSONResponse(apiresponse.Error("backup_item_not_found", err)), nil
	case err != nil:
		return nil, err
	}
	return api.DeleteWrappedBackupKey204Response{}, nil
}
