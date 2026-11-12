package linkpreview

import (
	"bytes"
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// Handler serves the link preview proxy.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func callerKey(ctx context.Context) (string, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return "", errNoPrincipal
	}
	return principal.UserID.String(), nil
}

// failure is the error response for err, or nil for an unexpected error.
type failure struct {
	status int
	body   api.Error
}

func failureOf(err error) *failure {
	switch {
	case errors.Is(err, ErrDisabled):
		return &failure{403, apiresponse.Error("preview_proxy_off", err)}
	case errors.Is(err, ErrInvalidLink):
		return &failure{422, apiresponse.Error("invalid_link", err)}
	case errors.Is(err, ErrRateLimited):
		return &failure{429, apiresponse.Error("rate_limited", err)}
	case errors.Is(err, ErrUnavailable):
		return &failure{502, apiresponse.Error("preview_unavailable", ErrUnavailable)}
	}
	return nil
}

// GetLinkPreview answers GET /api/v1/link-previews.
func (handler *Handler) GetLinkPreview(ctx context.Context, request api.GetLinkPreviewRequestObject) (api.GetLinkPreviewResponseObject, error) {
	key, err := callerKey(ctx)
	if err != nil {
		return nil, err
	}
	page, err := handler.service.Preview(ctx, key, request.Params.Url)
	if failed := failureOf(err); failed != nil {
		switch failed.status {
		case 403:
			return api.GetLinkPreview403JSONResponse(failed.body), nil
		case 422:
			return api.GetLinkPreview422JSONResponse(failed.body), nil
		case 429:
			return api.GetLinkPreview429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(failed.body)}, nil
		default:
			return api.GetLinkPreview502JSONResponse(failed.body), nil
		}
	}
	if err != nil {
		return nil, err
	}
	response := api.GetLinkPreview200JSONResponse{Title: page.Title, Description: page.Description}
	if page.ImageURL != "" {
		response.ImageUrl = &page.ImageURL
	}
	return response, nil
}

// GetLinkPreviewImage answers GET /api/v1/link-previews/image.
func (handler *Handler) GetLinkPreviewImage(ctx context.Context, request api.GetLinkPreviewImageRequestObject) (api.GetLinkPreviewImageResponseObject, error) {
	key, err := callerKey(ctx)
	if err != nil {
		return nil, err
	}
	image, err := handler.service.Image(ctx, key, request.Params.Url)
	if failed := failureOf(err); failed != nil {
		switch failed.status {
		case 403:
			return api.GetLinkPreviewImage403JSONResponse(failed.body), nil
		case 422:
			return api.GetLinkPreviewImage422JSONResponse(failed.body), nil
		case 429:
			return api.GetLinkPreviewImage429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(failed.body)}, nil
		default:
			return api.GetLinkPreviewImage502JSONResponse(failed.body), nil
		}
	}
	if err != nil {
		return nil, err
	}
	return api.GetLinkPreviewImage200ApplicationoctetStreamResponse{Body: bytes.NewReader(image.Data), ContentLength: int64(len(image.Data))}, nil
}
