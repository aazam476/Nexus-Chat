package oidc

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// requireAdmin returns the body of a 403 response if the caller is not an
// instance admin.
func (handler *Handler) requireAdmin(ctx context.Context) (*api.ForbiddenJSONResponse, error) {
	_, err := accounts.RequireInstanceAdmin(ctx, handler.users)
	if errors.Is(err, accounts.ErrForbidden) {
		forbidden := api.ForbiddenJSONResponse(apiresponse.Error("forbidden", err))
		return &forbidden, nil
	}
	return nil, err
}

// ListOidcProviders answers GET /api/v1/admin/oidc-providers.
func (handler *Handler) ListOidcProviders(ctx context.Context, _ api.ListOidcProvidersRequestObject) (api.ListOidcProvidersResponseObject, error) {
	forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.ListOidcProviders403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	providers, err := handler.providers.List(ctx)
	if err != nil {
		return nil, err
	}
	response := api.ListOidcProviders200JSONResponse{Providers: make([]api.OidcProvider, 0, len(providers))}
	for _, provider := range providers {
		response.Providers = append(response.Providers, adminProvider(provider))
	}
	base, err := handler.settings.BaseURL(ctx)
	switch {
	case err == nil:
		callback := base + CallbackPath
		response.CallbackUrl = &callback
	case !errors.Is(err, instanceconfig.ErrNoDomain):
		return nil, err
	}
	return response, nil
}

// CreateOidcProvider answers POST /api/v1/admin/oidc-providers.
func (handler *Handler) CreateOidcProvider(ctx context.Context, request api.CreateOidcProviderRequestObject) (api.CreateOidcProviderResponseObject, error) {
	forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.CreateOidcProvider403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	body := request.Body
	created, err := handler.providers.Create(ctx, ProviderInput{
		ID: body.Id, DisplayName: body.DisplayName, Issuer: body.Issuer, ClientID: body.ClientId,
		ClientSecret: body.ClientSecret, Scopes: scopes(body.Scopes), AllowSignup: body.AllowSignup,
	})
	var problems *validation.Error
	switch {
	case errors.As(err, &problems):
		return api.CreateOidcProvider422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	case errors.Is(err, ErrProviderExists):
		return api.CreateOidcProvider409JSONResponse(apiresponse.Error("provider_exists", err)), nil
	case err != nil:
		return nil, err
	}
	return api.CreateOidcProvider201JSONResponse(adminProvider(created)), nil
}

// UpdateOidcProvider answers PUT /api/v1/admin/oidc-providers/{providerId}.
func (handler *Handler) UpdateOidcProvider(ctx context.Context, request api.UpdateOidcProviderRequestObject) (api.UpdateOidcProviderResponseObject, error) {
	forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.UpdateOidcProvider403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	body := request.Body
	updated, err := handler.providers.Update(ctx, request.ProviderId, ProviderInput{
		DisplayName: body.DisplayName, Issuer: body.Issuer, ClientID: body.ClientId,
		ClientSecret: body.ClientSecret, Scopes: scopes(body.Scopes), AllowSignup: body.AllowSignup,
	})
	var problems *validation.Error
	switch {
	case errors.As(err, &problems):
		return api.UpdateOidcProvider422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	case errors.Is(err, ErrProviderNotFound):
		return api.UpdateOidcProvider404JSONResponse{ProviderNotFoundJSONResponse: api.ProviderNotFoundJSONResponse(apiresponse.Error("provider_not_found", err))}, nil
	case err != nil:
		return nil, err
	}
	return api.UpdateOidcProvider200JSONResponse(adminProvider(updated)), nil
}

// DeleteOidcProvider answers DELETE /api/v1/admin/oidc-providers/{providerId}.
func (handler *Handler) DeleteOidcProvider(ctx context.Context, request api.DeleteOidcProviderRequestObject) (api.DeleteOidcProviderResponseObject, error) {
	forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.DeleteOidcProvider403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	err = handler.providers.Delete(ctx, request.ProviderId)
	if errors.Is(err, ErrProviderNotFound) {
		return api.DeleteOidcProvider404JSONResponse{ProviderNotFoundJSONResponse: api.ProviderNotFoundJSONResponse(apiresponse.Error("provider_not_found", err))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.DeleteOidcProvider204Response{}, nil
}

func scopes(requested *[]string) []string {
	if requested == nil {
		return nil
	}
	return *requested
}

func adminProvider(provider ProviderView) api.OidcProvider {
	return api.OidcProvider{
		Id: provider.ID, DisplayName: provider.DisplayName, Issuer: provider.Issuer, ClientId: provider.ClientID,
		ClientSecretSet: provider.ClientSecretSet, Scopes: append([]string{}, provider.Scopes...), AllowSignup: provider.AllowSignup,
	}
}
