package setup

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// Handler serves the setup endpoints. It translates between the generated
// API types and the service; all rules live in the service.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler for service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetSetupStatus answers GET /api/v1/setup.
func (handler *Handler) GetSetupStatus(ctx context.Context, _ api.GetSetupStatusRequestObject) (api.GetSetupStatusResponseObject, error) {
	status, err := handler.service.Status(ctx)
	if err != nil {
		return nil, err
	}
	response := api.GetSetupStatus200JSONResponse{
		SetupRequired:         status.Required,
		SecureConnection:      clientip.FromContext(ctx).SecureContext(),
		DomainFromEnvironment: status.DomainFromEnvironment,
	}
	if status.Domain != "" {
		response.Domain = &status.Domain
	}
	return response, nil
}

// CompleteSetup answers POST /api/v1/setup.
func (handler *Handler) CompleteSetup(ctx context.Context, request api.CompleteSetupRequestObject) (api.CompleteSetupResponseObject, error) {
	body := request.Body
	serviceRequest := Request{
		Token: body.SetupToken,
		Admin: accounts.NewAccount{
			Username: body.Admin.Username, DisplayName: body.Admin.DisplayName,
			Email: body.Admin.Email, Password: body.Admin.Password,
		},
		InstanceName:     body.Instance.Name,
		RegistrationMode: string(body.Instance.RegistrationMode),
	}
	if body.Instance.Domain != nil {
		serviceRequest.Domain = *body.Instance.Domain
	}

	adminID, err := handler.service.Complete(ctx, serviceRequest, clientip.FromContext(ctx).Address.String())
	if err != nil {
		return completeSetupError(err)
	}
	return api.CompleteSetup201JSONResponse{AdminUserId: adminID}, nil
}

func completeSetupError(err error) (api.CompleteSetupResponseObject, error) {
	var problems *validation.Error
	switch {
	case errors.Is(err, ErrRateLimited):
		return api.CompleteSetup429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case errors.Is(err, ErrInvalidToken):
		return api.CompleteSetup401JSONResponse(apiresponse.Error("invalid_setup_token", err)), nil
	case errors.Is(err, ErrAlreadyComplete):
		return api.CompleteSetup409JSONResponse(apiresponse.Error("setup_already_complete", err)), nil
	case errors.As(err, &problems):
		return api.CompleteSetup422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	default:
		// Unexpected errors become a 500 in the shared error handler,
		// which logs them without echoing details to the client.
		return nil, err
	}
}
