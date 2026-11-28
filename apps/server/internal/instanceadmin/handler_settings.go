package instanceadmin

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// GetRegistrationSettings answers GET /api/v1/admin/registration.
func (handler *Handler) GetRegistrationSettings(ctx context.Context, _ api.GetRegistrationSettingsRequestObject) (api.GetRegistrationSettingsResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.GetRegistrationSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	mode, err := handler.service.Settings().RegistrationMode(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetRegistrationSettings200JSONResponse{Mode: api.RegistrationSettingsMode(mode)}, nil
}

// UpdateRegistrationSettings answers PUT /api/v1/admin/registration.
func (handler *Handler) UpdateRegistrationSettings(ctx context.Context, request api.UpdateRegistrationSettingsRequestObject) (api.UpdateRegistrationSettingsResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.UpdateRegistrationSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	err = handler.service.Settings().SetRegistrationMode(ctx, string(request.Body.Mode))
	if errors.Is(err, instanceconfig.ErrInvalidRegistrationMode) {
		return api.UpdateRegistrationSettings422JSONResponse(apiresponse.Error("invalid", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateRegistrationSettings204Response{}, nil
}

// GetFederationSettings answers GET /api/v1/admin/federation.
func (handler *Handler) GetFederationSettings(ctx context.Context, _ api.GetFederationSettingsRequestObject) (api.GetFederationSettingsResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.GetFederationSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	policy, err := handler.service.Settings().Federation(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetFederationSettings200JSONResponse{Mode: api.FederationPolicyMode(policy.Mode), Allowed: policy.Allowed, Blocked: policy.Blocked}, nil
}

// UpdateFederationSettings answers PUT /api/v1/admin/federation.
func (handler *Handler) UpdateFederationSettings(ctx context.Context, request api.UpdateFederationSettingsRequestObject) (api.UpdateFederationSettingsResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.UpdateFederationSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	body := request.Body
	err = handler.service.Settings().SetFederation(ctx, instanceconfig.FederationPolicy{Mode: string(body.Mode), Allowed: body.Allowed, Blocked: body.Blocked})
	if errors.Is(err, instanceconfig.ErrInvalidFederation) {
		return api.UpdateFederationSettings422JSONResponse(apiresponse.Error("invalid", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateFederationSettings204Response{}, nil
}

// GetModerationSettings answers GET /api/v1/admin/moderation.
func (handler *Handler) GetModerationSettings(ctx context.Context, _ api.GetModerationSettingsRequestObject) (api.GetModerationSettingsResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.GetModerationSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	enabled, err := handler.service.Settings().ReportsToAdmins(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetModerationSettings200JSONResponse{ReportsToAdmins: enabled}, nil
}

// UpdateModerationSettings answers PUT /api/v1/admin/moderation.
func (handler *Handler) UpdateModerationSettings(ctx context.Context, request api.UpdateModerationSettingsRequestObject) (api.UpdateModerationSettingsResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.UpdateModerationSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err := handler.service.Settings().SetReportsToAdmins(ctx, request.Body.ReportsToAdmins); err != nil {
		return nil, err
	}
	return api.UpdateModerationSettings204Response{}, nil
}
