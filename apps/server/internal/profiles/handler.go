package profiles

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
	"github.com/aazam476/nexus-chat/apps/server/internal/realtime"
)

// avatarCacheControl lets browsers keep a picture forever: a new picture
// always has a new address.
const avatarCacheControl = "public, max-age=31536000, immutable"

var (
	errNoPrincipal = errors.New("handler requires an authenticated request")
	errPartialCrop = errors.New("give cropX, cropY and cropSize together, or none of them")
)

type presenceReader interface {
	PresenceOf(user store.User) realtime.Presence
}

// Handler serves the profile endpoints.
type Handler struct {
	service  *Service
	presence presenceReader
}

// NewHandler returns a Handler for service. presence tells what status
// others see.
func NewHandler(service *Service, presence presenceReader) *Handler {
	return &Handler{service: service, presence: presence}
}

func principal(ctx context.Context) (session.Principal, error) {
	found, ok := session.PrincipalFrom(ctx)
	if !ok {
		return session.Principal{}, errNoPrincipal
	}
	return found, nil
}

// UpdateCurrentUser answers PATCH /api/v1/users/me.
func (handler *Handler) UpdateCurrentUser(ctx context.Context, request api.UpdateCurrentUserRequestObject) (api.UpdateCurrentUserResponseObject, error) {
	caller, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	user, err := handler.service.UpdateProfile(ctx, caller.UserID, ProfileUpdate{DisplayName: request.Body.DisplayName, Bio: request.Body.Bio})
	var problems *validation.Error
	if errors.As(err, &problems) {
		return api.UpdateCurrentUser422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateCurrentUser200JSONResponse(accounts.CurrentUserResponse(user)), nil
}

// SetStatus answers PUT /api/v1/users/me/status.
func (handler *Handler) SetStatus(ctx context.Context, request api.SetStatusRequestObject) (api.SetStatusResponseObject, error) {
	caller, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	update := StatusUpdate{Status: string(request.Body.Status)}
	if custom := request.Body.CustomStatus; custom != nil {
		update.CustomText, update.ExpiresAt = custom.Text, custom.ExpiresAt
	}
	user, err := handler.service.SetStatus(ctx, caller.UserID, update)
	var problems *validation.Error
	if errors.As(err, &problems) {
		return api.SetStatus422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.SetStatus200JSONResponse(accounts.CurrentUserResponse(user)), nil
}

// SetAvatar answers PUT /api/v1/users/me/avatar.
func (handler *Handler) SetAvatar(ctx context.Context, request api.SetAvatarRequestObject) (api.SetAvatarResponseObject, error) {
	caller, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	crop, err := cropFrom(request.Params)
	if err != nil {
		return api.SetAvatar422JSONResponse(apiresponse.Error("invalid_crop", err)), nil
	}
	// One byte more than allowed tells a too-large file from one exactly at
	// the limit without reading the rest.
	upload, err := io.ReadAll(io.LimitReader(request.Body, avatar.MaximumUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading picture: %w", err)
	}
	user, err := handler.service.SetAvatar(ctx, caller.UserID, upload, crop)
	switch {
	case errors.Is(err, avatar.ErrTooLarge):
		return api.SetAvatar413JSONResponse(apiresponse.Error("image_too_large", err)), nil
	case errors.Is(err, avatar.ErrBadCrop):
		return api.SetAvatar422JSONResponse(apiresponse.Error("invalid_crop", err)), nil
	case errors.Is(err, avatar.ErrUnsupported), errors.Is(err, avatar.ErrDamaged):
		return api.SetAvatar422JSONResponse(apiresponse.Error("unsupported_image", err)), nil
	case errors.Is(err, ErrRateLimited):
		return api.SetAvatar429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case err != nil:
		return nil, err
	}
	return api.SetAvatar200JSONResponse(accounts.CurrentUserResponse(user)), nil
}

func cropFrom(params api.SetAvatarParams) (*avatar.Crop, error) {
	given := 0
	for _, value := range []*int{params.CropX, params.CropY, params.CropSize} {
		if value != nil {
			given++
		}
	}
	switch given {
	case 0:
		return nil, nil
	case 3:
		return &avatar.Crop{X: *params.CropX, Y: *params.CropY, Size: *params.CropSize}, nil
	default:
		return nil, errPartialCrop
	}
}

// RemoveAvatar answers DELETE /api/v1/users/me/avatar.
func (handler *Handler) RemoveAvatar(ctx context.Context, _ api.RemoveAvatarRequestObject) (api.RemoveAvatarResponseObject, error) {
	caller, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := handler.service.RemoveAvatar(ctx, caller.UserID); err != nil {
		return nil, err
	}
	return api.RemoveAvatar204Response{}, nil
}

// GetUserProfile answers GET /api/v1/users/{userId}.
func (handler *Handler) GetUserProfile(ctx context.Context, request api.GetUserProfileRequestObject) (api.GetUserProfileResponseObject, error) {
	user, err := handler.service.Get(ctx, request.UserId)
	if errors.Is(err, ErrUserNotFound) {
		return api.GetUserProfile404JSONResponse(apiresponse.Error("user_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	presence := handler.presence.PresenceOf(user)
	return api.GetUserProfile200JSONResponse{
		Id: user.ID, Username: user.Username, DisplayName: user.DisplayName, Bio: user.Bio, AvatarUrl: accounts.AvatarURL(user),
		Status: api.PresenceStatus(presence.Status), CustomStatus: presence.CustomStatus,
	}, nil
}

// GetAvatar answers GET /api/v1/avatars/{avatarId}. The generated code
// closes the file after copying it to the response.
func (handler *Handler) GetAvatar(ctx context.Context, request api.GetAvatarRequestObject) (api.GetAvatarResponseObject, error) {
	picture, err := handler.service.OpenAvatar(ctx, request.AvatarId)
	if errors.Is(err, ErrAvatarNotFound) {
		return api.GetAvatar404JSONResponse(apiresponse.Error("avatar_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	body := picture.Content
	headers := api.GetAvatar200ResponseHeaders{CacheControl: avatarCacheControl}
	switch picture.ContentType {
	case "image/gif":
		return api.GetAvatar200ImagegifResponse{Body: body, Headers: headers}, nil
	case "image/jpeg":
		return api.GetAvatar200ImagejpegResponse{Body: body, Headers: headers}, nil
	default:
		return api.GetAvatar200ImagepngResponse{Body: body, Headers: headers}, nil
	}
}
