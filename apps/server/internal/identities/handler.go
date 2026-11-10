package identities

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// Handler serves the identity endpoints.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// PublishIdentity answers PUT /api/v1/identity.
func (handler *Handler) PublishIdentity(ctx context.Context, request api.PublishIdentityRequestObject) (api.PublishIdentityResponseObject, error) {
	caller, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.service.Publish(ctx, caller, Identity{
		MasterPublicKey:      request.Body.MasterPublicKey,
		SelfSigningPublicKey: request.Body.SelfSigningPublicKey,
		SelfSigningSignature: request.Body.SelfSigningSignature,
	})
	switch {
	case errors.Is(err, ErrInvalidIdentity):
		return api.PublishIdentity422JSONResponse(apiresponse.Error("invalid_identity", err)), nil
	case errors.Is(err, ErrInvalidSignature):
		return api.PublishIdentity422JSONResponse(apiresponse.Error("invalid_signature", err)), nil
	case errors.Is(err, session.ErrReauthenticationRequired):
		return api.PublishIdentity403JSONResponse{
			ReauthenticationRequiredJSONResponse: api.ReauthenticationRequiredJSONResponse(apiresponse.Error("reauthentication_required", err)),
		}, nil
	case err != nil:
		return nil, err
	}
	return api.PublishIdentity204Response{}, nil
}

// GetIdentity answers GET /api/v1/users/{userId}/identity.
func (handler *Handler) GetIdentity(ctx context.Context, request api.GetIdentityRequestObject) (api.GetIdentityResponseObject, error) {
	identity, err := handler.service.Get(ctx, request.UserId)
	if errors.Is(err, ErrNotFound) {
		return api.GetIdentity404JSONResponse(apiresponse.Error("identity_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetIdentity200JSONResponse{
		MasterPublicKey:      identity.MasterPublicKey,
		SelfSigningPublicKey: identity.SelfSigningPublicKey,
		SelfSigningSignature: identity.SelfSigningSignature,
	}, nil
}
