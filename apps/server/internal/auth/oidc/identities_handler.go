package oidc

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// CompleteProviderLink answers POST /api/v1/auth/oidc/link/complete.
func (handler *Handler) CompleteProviderLink(ctx context.Context, request api.CompleteProviderLinkRequestObject) (api.CompleteProviderLinkResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.signIn.CompleteLink(ctx, principal.UserID, request.Body.FlowId, request.Body.Code)
	switch {
	case err == nil:
		return api.CompleteProviderLink204Response{}, nil
	case errors.Is(err, ErrInvalidFlow):
		return api.CompleteProviderLink401JSONResponse(apiresponse.Error("invalid_flow", err)), nil
	case errors.Is(err, ErrIdentityInUse):
		return api.CompleteProviderLink409JSONResponse(apiresponse.Error("identity_in_use", err)), nil
	case errors.Is(err, ErrAlreadyLinked):
		return api.CompleteProviderLink409JSONResponse(apiresponse.Error("already_linked", err)), nil
	default:
		return nil, err
	}
}

// ListLinkedProviders answers GET /api/v1/auth/oidc/identities.
func (handler *Handler) ListLinkedProviders(ctx context.Context, _ api.ListLinkedProvidersRequestObject) (api.ListLinkedProvidersResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	identities, err := handler.signIn.Identities(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	response := make(api.ListLinkedProviders200JSONResponse, 0, len(identities))
	for _, identity := range identities {
		response = append(response, api.LinkedProvider{
			ProviderId: identity.ProviderID, DisplayName: identity.DisplayName, Email: identity.Email, LinkedAt: identity.LinkedAt,
		})
	}
	return response, nil
}

// UnlinkProvider answers DELETE /api/v1/auth/oidc/identities/{providerId}.
func (handler *Handler) UnlinkProvider(ctx context.Context, request api.UnlinkProviderRequestObject) (api.UnlinkProviderResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.signIn.Unlink(ctx, principal.UserID, request.ProviderId)
	switch {
	case err == nil:
		return api.UnlinkProvider204Response{}, nil
	case errors.Is(err, ErrNotLinked):
		return api.UnlinkProvider404JSONResponse(apiresponse.Error("not_linked", err)), nil
	case errors.Is(err, ErrLastLoginMethod):
		return api.UnlinkProvider409JSONResponse(apiresponse.Error("last_login_method", err)), nil
	default:
		return nil, err
	}
}
