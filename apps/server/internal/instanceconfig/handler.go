package instanceconfig

import (
	"context"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
)

// Handler serves the public instance information.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler reading settings from service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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
	response := api.GetInstanceInfo200JSONResponse{
		Name:             name,
		RegistrationMode: api.RegistrationMode(mode),
		SecureConnection: clientip.FromContext(ctx).SecureContext(),
	}
	if domain != "" {
		response.Domain = &domain
	}
	return response, nil
}
