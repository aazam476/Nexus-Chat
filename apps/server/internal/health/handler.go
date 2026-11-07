package health

import (
	"context"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// Handler serves the health endpoints. It implements the health operations
// of api.StrictServerInterface and contains no logic of its own.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler that reports the results of service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetLiveness answers GET /healthz. Reaching this code proves liveness.
func (handler *Handler) GetLiveness(_ context.Context, _ api.GetLivenessRequestObject) (api.GetLivenessResponseObject, error) {
	return api.GetLiveness200JSONResponse{Status: api.HealthStateOk, Checks: []api.HealthCheck{}}, nil
}

// GetReadiness answers GET /readyz with 200 when every dependency is
// reachable and 503 otherwise.
func (handler *Handler) GetReadiness(ctx context.Context, _ api.GetReadinessRequestObject) (api.GetReadinessResponseObject, error) {
	allOK, results := handler.service.Readiness(ctx)
	checks := make([]api.HealthCheck, 0, len(results))
	for _, result := range results {
		checks = append(checks, api.HealthCheck{Name: result.Name, Status: state(result.OK)})
	}
	if !allOK {
		return api.GetReadiness503JSONResponse{Status: api.HealthStateUnavailable, Checks: checks}, nil
	}
	return api.GetReadiness200JSONResponse{Status: api.HealthStateOk, Checks: checks}, nil
}

func state(ok bool) api.HealthState {
	if ok {
		return api.HealthStateOk
	}
	return api.HealthStateUnavailable
}
