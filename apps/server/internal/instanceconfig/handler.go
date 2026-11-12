package instanceconfig

import (
	"context"
	"crypto/ed25519"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
)

// Handler serves the public instance information.
type Handler struct {
	service    *Service
	signingKey ed25519.PublicKey
	limits     Limits
}

// Limits are instance settings from .env that clients need to know.
type Limits struct {
	// GroupDMMaxMembers is the largest group DM (GROUP_DM_MAX_MEMBERS).
	GroupDMMaxMembers int
}

// NewHandler returns a Handler reading settings from service.
// signingKey is the instance's public key, which members' devices use to
// check membership directives (docs/e2ee-design.md §3.2).
func NewHandler(service *Service, signingKey ed25519.PublicKey, limits Limits) *Handler {
	return &Handler{service: service, signingKey: signingKey, limits: limits}
}

// GetInstanceInfo answers GET /api/v1/instance. secureConnection is what
// clients use to decide whether to show the HTTPS warning.
func (handler *Handler) GetInstanceInfo(ctx context.Context, _ api.GetInstanceInfoRequestObject) (api.GetInstanceInfoResponseObject, error) {
	name, err := handler.service.InstanceName(ctx)
	if err != nil {
		return nil, err
	}
	mode, err := handler.service.RegistrationMode(ctx)
	if err != nil {
		return nil, err
	}
	domain, err := handler.service.Domain(ctx)
	if err != nil {
		return nil, err
	}
	attachmentLimit, err := handler.service.AttachmentMaxBytes(ctx)
	if err != nil {
		return nil, err
	}
	response := api.GetInstanceInfo200JSONResponse{
		Name:             name,
		RegistrationMode: api.RegistrationMode(mode),
		SecureConnection: clientip.FromContext(ctx).SecureContext(),
		SigningKey:       handler.signingKey,
		Limits: api.InstanceLimits{
			GroupDmMaxMembers: handler.limits.GroupDMMaxMembers, AttachmentMaxBytes: attachmentLimit,
		},
	}
	if domain != "" {
		response.Domain = &domain
	}
	return response, nil
}
