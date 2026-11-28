package instanceadmin

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers"
)

// Handler serves the admin panel's endpoints.
type Handler struct {
	service *Service
	users   accounts.UserReader
}

// NewHandler returns a Handler; users is used to check that the caller is an instance admin.
func NewHandler(service *Service, users accounts.UserReader) *Handler {
	return &Handler{service: service, users: users}
}

// admin returns the calling admin's ID, or the 403 to answer with.
func (handler *Handler) admin(ctx context.Context) (uuid.UUID, *api.ForbiddenJSONResponse, error) {
	user, err := accounts.RequireInstanceAdmin(ctx, handler.users)
	if errors.Is(err, accounts.ErrForbidden) {
		forbidden := api.ForbiddenJSONResponse(apiresponse.Error("forbidden", err))
		return uuid.Nil, &forbidden, nil
	}
	return user.ID, nil, err
}

// refusal is the API code for an error the caller can act on.
func refusal(err error) (string, bool) {
	codes := []struct {
		err  error
		code string
	}{
		{ErrNoUser, "user_not_found"}, {ErrNotSuspended, "not_suspended"}, {ErrSelf, "self"},
		{ErrLastAdmin, "last_admin"}, {ErrInvalid, "invalid"}, {servers.ErrNotFound, "server_not_found"},
	}
	for _, known := range codes {
		if errors.Is(err, known.err) {
			return known.code, true
		}
	}
	return "", false
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// ListAdminUsers answers GET /api/v1/admin/users.
func (handler *Handler) ListAdminUsers(ctx context.Context, request api.ListAdminUsersRequestObject) (api.ListAdminUsersResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.ListAdminUsers403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	users, err := handler.service.Users(ctx, text(request.Params.Q), text(request.Params.After))
	if errors.Is(err, ErrInvalid) {
		return api.ListAdminUsers422JSONResponse(apiresponse.Error("invalid", err)), nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListAdminUsers200JSONResponse, 0, len(users))
	for _, user := range users {
		out = append(out, api.AdminUser{
			Id: user.ID, Username: user.Username, DisplayName: user.DisplayName, Email: user.Email, IsAdmin: user.IsInstanceAdmin,
			CreatedAt: user.CreatedAt, SuspendedAt: user.SuspendedAt, SuspensionReason: user.SuspensionReason,
		})
	}
	return out, nil
}

// userAction runs an action on an account and answers 204, 403, 404 or 422.
func userAction[R any](ctx context.Context, handler *Handler, act func(adminID uuid.UUID) error, answers userAnswers[R]) (R, error) {
	var zero R
	adminID, forbidden, err := handler.admin(ctx)
	if err != nil {
		return zero, err
	}
	if forbidden != nil {
		return answers.forbidden(*forbidden), nil
	}
	err = act(adminID)
	if code, ok := refusal(err); ok {
		body := apiresponse.Error(code, err)
		if errors.Is(err, ErrNoUser) || errors.Is(err, ErrNotSuspended) {
			return answers.notFound(body), nil
		}
		return answers.invalid(body), nil
	}
	if err != nil {
		return zero, err
	}
	return answers.done, nil
}

// userAnswers are one operation's responses.
type userAnswers[R any] struct {
	done      R
	forbidden func(api.ForbiddenJSONResponse) R
	notFound  func(api.Error) R
	invalid   func(api.Error) R
}
