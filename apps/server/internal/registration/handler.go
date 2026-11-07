package registration

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// Handler serves registration and the admin invite endpoints.
type Handler struct {
	service *Service
	invites *Invites
	users   accounts.UserReader
}

// NewHandler returns a Handler.
func NewHandler(service *Service, invites *Invites, users accounts.UserReader) *Handler {
	return &Handler{service: service, invites: invites, users: users}
}

// Register answers POST /api/v1/auth/register.
func (handler *Handler) Register(ctx context.Context, request api.RegisterRequestObject) (api.RegisterResponseObject, error) {
	body := request.Body
	serviceRequest := Request{
		Account: accounts.NewAccount{Username: body.Username, DisplayName: body.DisplayName, Email: body.Email, Password: body.Password},
		Device:  session.Device{Name: apiresponse.DeviceName(body.DeviceName)}, ClientKey: clientip.FromContext(ctx).Address.String(),
	}
	if body.InviteCode != nil {
		serviceRequest.InviteCode = *body.InviteCode
	}
	result, err := handler.service.Register(ctx, serviceRequest)
	var problems *validation.Error
	switch {
	case errors.Is(err, ErrRegistrationClosed):
		return api.Register403JSONResponse(apiresponse.Error("registration_closed", err)), nil
	case errors.Is(err, ErrInviteRequired):
		return api.Register403JSONResponse(apiresponse.Error("invite_required", err)), nil
	case errors.Is(err, ErrInvalidInvite):
		return api.Register403JSONResponse(apiresponse.Error("invalid_invite", err)), nil
	case errors.Is(err, ErrRateLimited):
		return api.Register429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case errors.As(err, &problems):
		return api.Register422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	case err != nil:
		return nil, err
	}
	return api.Register201JSONResponse{
		Token: result.Token, Session: apiresponse.Session(result.Session, result.Session.ID), User: accounts.CurrentUserResponse(result.User),
	}, nil
}
