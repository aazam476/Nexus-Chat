package social

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// ListBlocked answers GET /api/v1/blocks.
func (handler *Handler) ListBlocked(ctx context.Context, _ api.ListBlockedRequestObject) (api.ListBlockedResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	blocked, err := handler.service.ListBlocked(ctx, me)
	if err != nil {
		return nil, err
	}
	return api.ListBlocked200JSONResponse(contacts(blocked)), nil
}

// BlockUser answers PUT /api/v1/blocks/{userId}.
func (handler *Handler) BlockUser(ctx context.Context, request api.BlockUserRequestObject) (api.BlockUserResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.Block(ctx, me, request.UserId)
	switch {
	case errors.Is(err, ErrSelf):
		return api.BlockUser422JSONResponse(apiresponse.Error("cannot_block_self", err)), nil
	case errors.Is(err, ErrUserNotFound):
		return api.BlockUser404JSONResponse(apiresponse.Error("user_not_found", err)), nil
	case err != nil:
		return nil, err
	}
	return api.BlockUser204Response{}, nil
}

// UnblockUser answers DELETE /api/v1/blocks/{userId}.
func (handler *Handler) UnblockUser(ctx context.Context, request api.UnblockUserRequestObject) (api.UnblockUserResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.Unblock(ctx, me, request.UserId)
	if errors.Is(err, ErrNotFound) {
		return api.UnblockUser404JSONResponse(apiresponse.Error("not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.UnblockUser204Response{}, nil
}

// GetPrivacySettings answers GET /api/v1/privacy.
func (handler *Handler) GetPrivacySettings(ctx context.Context, _ api.GetPrivacySettingsRequestObject) (api.GetPrivacySettingsResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	privacy, err := handler.service.GetPrivacy(ctx, me)
	if err != nil {
		return nil, err
	}
	return api.GetPrivacySettings200JSONResponse{
		WhoCanDirectMessage: api.PrivacySettingsWhoCanDirectMessage(privacy.WhoCanDirectMessage),
		WhoCanFriendRequest: api.PrivacySettingsWhoCanFriendRequest(privacy.WhoCanFriendRequest),
	}, nil
}

// UpdatePrivacySettings answers PUT /api/v1/privacy.
func (handler *Handler) UpdatePrivacySettings(ctx context.Context, request api.UpdatePrivacySettingsRequestObject) (api.UpdatePrivacySettingsResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.SetPrivacy(ctx, me, Privacy{
		WhoCanDirectMessage: string(request.Body.WhoCanDirectMessage), WhoCanFriendRequest: string(request.Body.WhoCanFriendRequest),
	})
	if errors.Is(err, ErrInvalidPrivacy) {
		return api.UpdatePrivacySettings400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(apiresponse.Error("bad_request", err))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdatePrivacySettings204Response{}, nil
}
