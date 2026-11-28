package instanceadmin

import (
	"context"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// SuspendUser answers PUT /api/v1/admin/users/{userId}/suspension.
func (handler *Handler) SuspendUser(ctx context.Context, request api.SuspendUserRequestObject) (api.SuspendUserResponseObject, error) {
	reason := ""
	if request.Body != nil {
		reason = text(request.Body.Reason)
	}
	return userAction(ctx, handler, func(adminID uuid.UUID) error {
		return handler.service.Suspend(ctx, adminID, request.UserId, reason)
	}, userAnswers[api.SuspendUserResponseObject]{
		done: api.SuspendUser204Response{},
		forbidden: func(body api.ForbiddenJSONResponse) api.SuspendUserResponseObject {
			return api.SuspendUser403JSONResponse{ForbiddenJSONResponse: body}
		},
		notFound: func(body api.Error) api.SuspendUserResponseObject { return api.SuspendUser404JSONResponse(body) },
		invalid:  func(body api.Error) api.SuspendUserResponseObject { return api.SuspendUser422JSONResponse(body) },
	})
}

// ReinstateUser answers DELETE /api/v1/admin/users/{userId}/suspension.
func (handler *Handler) ReinstateUser(ctx context.Context, request api.ReinstateUserRequestObject) (api.ReinstateUserResponseObject, error) {
	return userAction(ctx, handler, func(adminID uuid.UUID) error {
		return handler.service.Reinstate(ctx, adminID, request.UserId)
	}, userAnswers[api.ReinstateUserResponseObject]{
		done: api.ReinstateUser204Response{},
		forbidden: func(body api.ForbiddenJSONResponse) api.ReinstateUserResponseObject {
			return api.ReinstateUser403JSONResponse{ForbiddenJSONResponse: body}
		},
		notFound: func(body api.Error) api.ReinstateUserResponseObject { return api.ReinstateUser404JSONResponse(body) },
		invalid:  func(body api.Error) api.ReinstateUserResponseObject { return api.ReinstateUser422JSONResponse(body) },
	})
}

// GrantInstanceAdmin answers PUT /api/v1/admin/users/{userId}/admin.
func (handler *Handler) GrantInstanceAdmin(ctx context.Context, request api.GrantInstanceAdminRequestObject) (api.GrantInstanceAdminResponseObject, error) {
	return userAction(ctx, handler, func(adminID uuid.UUID) error {
		return handler.service.SetAdmin(ctx, adminID, request.UserId, true)
	}, userAnswers[api.GrantInstanceAdminResponseObject]{
		done: api.GrantInstanceAdmin204Response{},
		forbidden: func(body api.ForbiddenJSONResponse) api.GrantInstanceAdminResponseObject {
			return api.GrantInstanceAdmin403JSONResponse{ForbiddenJSONResponse: body}
		},
		notFound: func(body api.Error) api.GrantInstanceAdminResponseObject {
			return api.GrantInstanceAdmin404JSONResponse(body)
		},
		invalid: func(body api.Error) api.GrantInstanceAdminResponseObject {
			return api.GrantInstanceAdmin422JSONResponse(body)
		},
	})
}

// RevokeInstanceAdmin answers DELETE /api/v1/admin/users/{userId}/admin.
func (handler *Handler) RevokeInstanceAdmin(ctx context.Context, request api.RevokeInstanceAdminRequestObject) (api.RevokeInstanceAdminResponseObject, error) {
	return userAction(ctx, handler, func(adminID uuid.UUID) error {
		return handler.service.SetAdmin(ctx, adminID, request.UserId, false)
	}, userAnswers[api.RevokeInstanceAdminResponseObject]{
		done: api.RevokeInstanceAdmin204Response{},
		forbidden: func(body api.ForbiddenJSONResponse) api.RevokeInstanceAdminResponseObject {
			return api.RevokeInstanceAdmin403JSONResponse{ForbiddenJSONResponse: body}
		},
		notFound: func(body api.Error) api.RevokeInstanceAdminResponseObject {
			return api.RevokeInstanceAdmin404JSONResponse(body)
		},
		invalid: func(body api.Error) api.RevokeInstanceAdminResponseObject {
			return api.RevokeInstanceAdmin422JSONResponse(body)
		},
	})
}
