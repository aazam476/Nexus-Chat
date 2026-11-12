package attachments

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// downloadCacheControl keeps encrypted files out of shared caches; a
// device may keep its own copy, since a file never changes.
const downloadCacheControl = "private, max-age=31536000, immutable"

// Handler serves the attachment endpoints.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func caller(ctx context.Context) (uuid.UUID, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return uuid.Nil, errNoPrincipal
	}
	return principal.UserID, nil
}

// UploadAttachment answers POST /api/v1/conversations/{conversationId}/attachments.
func (handler *Handler) UploadAttachment(ctx context.Context, request api.UploadAttachmentRequestObject) (api.UploadAttachmentResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	stored, err := handler.service.Upload(ctx, me, Upload{ConversationID: request.ConversationId, Size: request.Params.Size, Content: request.Body})
	switch {
	case errors.Is(err, ErrNotAllowed):
		return api.UploadAttachment403JSONResponse(apiresponse.Error("not_allowed", err)), nil
	case errors.Is(err, ErrTooLarge):
		return api.UploadAttachment413JSONResponse(apiresponse.Error("attachment_too_large", err)), nil
	case errors.Is(err, ErrInvalidSize):
		return api.UploadAttachment422JSONResponse(apiresponse.Error("invalid_size", err)), nil
	case errors.Is(err, ErrRateLimited):
		return api.UploadAttachment429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case err != nil:
		return nil, err
	}
	return api.UploadAttachment201JSONResponse{Id: stored.ID, Size: stored.Size}, nil
}

// DownloadAttachment answers GET /api/v1/conversations/{conversationId}/attachments/{attachmentId}.
// The generated code closes the file after copying it to the response.
func (handler *Handler) DownloadAttachment(ctx context.Context, request api.DownloadAttachmentRequestObject) (api.DownloadAttachmentResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	download, err := handler.service.Open(ctx, me, request.ConversationId, request.AttachmentId)
	if errors.Is(err, ErrNotFound) {
		return api.DownloadAttachment404JSONResponse(apiresponse.Error("attachment_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.DownloadAttachment200ApplicationoctetStreamResponse{
		Body: download.Content, ContentLength: download.Size,
		Headers: api.DownloadAttachment200ResponseHeaders{CacheControl: downloadCacheControl},
	}, nil
}
