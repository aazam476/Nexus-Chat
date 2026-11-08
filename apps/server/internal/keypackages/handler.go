package keypackages

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

// Handler serves the key-package endpoints.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func stockResponse(stock Stock) api.KeyPackageStock {
	return api.KeyPackageStock{OneTime: int32(stock.OneTime), HasLastResort: stock.HasLastResort, Refill: stock.Refill} //nolint:gosec // at most MaximumOneTime
}

// GetKeyPackageStock answers GET /api/v1/key-packages.
func (handler *Handler) GetKeyPackageStock(ctx context.Context, _ api.GetKeyPackageStockRequestObject) (api.GetKeyPackageStockResponseObject, error) {
	device, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	stock, err := handler.service.Count(ctx, device)
	if err != nil {
		return nil, err
	}
	return api.GetKeyPackageStock200JSONResponse(stockResponse(stock)), nil
}

// UploadKeyPackages answers POST /api/v1/key-packages.
func (handler *Handler) UploadKeyPackages(ctx context.Context, request api.UploadKeyPackagesRequestObject) (api.UploadKeyPackagesResponseObject, error) {
	device, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	var lastResort []byte
	if request.Body.LastResort != nil {
		lastResort = *request.Body.LastResort
	}
	stock, err := handler.service.Upload(ctx, device, request.Body.KeyPackages, lastResort)
	switch {
	case errors.Is(err, ErrInvalidPackage):
		return api.UploadKeyPackages422JSONResponse(apiresponse.Error("invalid_key_package", err)), nil
	case errors.Is(err, ErrEmptyUpload), errors.Is(err, ErrBatchTooLarge):
		return api.UploadKeyPackages422JSONResponse(apiresponse.Error("validation_failed", err)), nil
	case errors.Is(err, ErrStockFull):
		return api.UploadKeyPackages409JSONResponse(apiresponse.Error("key_package_stock_full", err)), nil
	case err != nil:
		return nil, err
	}
	return api.UploadKeyPackages200JSONResponse(stockResponse(stock)), nil
}

// ClaimKeyPackages answers POST /api/v1/users/{userId}/key-packages/claim.
func (handler *Handler) ClaimKeyPackages(ctx context.Context, request api.ClaimKeyPackagesRequestObject) (api.ClaimKeyPackagesResponseObject, error) {
	claimer, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	claimed, err := handler.service.Claim(ctx, claimer, request.UserId)
	if errors.Is(err, ErrRateLimited) {
		return api.ClaimKeyPackages429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	}
	if err != nil {
		return nil, err
	}
	response := make(api.ClaimKeyPackages200JSONResponse, 0, len(claimed))
	for _, item := range claimed {
		response = append(response, api.ClaimedKeyPackage{DeviceId: item.DeviceID, KeyPackage: item.KeyPackage, LastResort: item.LastResort})
	}
	return response, nil
}
