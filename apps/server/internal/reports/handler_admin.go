package reports

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// forbidden is the 403 answer to someone who is not an instance admin.
func forbidden(err error) api.ForbiddenJSONResponse {
	return api.ForbiddenJSONResponse(apiresponse.Error("forbidden", err))
}

// ListAdminReports answers GET /api/v1/admin/reports.
func (handler *Handler) ListAdminReports(ctx context.Context, request api.ListAdminReportsRequestObject) (api.ListAdminReportsResponseObject, error) {
	_, err := accounts.RequireInstanceAdmin(ctx, handler.users)
	if errors.Is(err, accounts.ErrForbidden) {
		return api.ListAdminReports403JSONResponse{ForbiddenJSONResponse: forbidden(err)}, nil
	}
	if err != nil {
		return nil, err
	}
	list, err := handler.service.AdminReports(ctx, statusOf(request.Params.Status))
	if err != nil {
		return nil, err
	}
	return api.ListAdminReports200JSONResponse(reportResponses(list)), nil
}

// ResolveAdminReport answers POST /api/v1/admin/reports/{reportId}/resolve.
func (handler *Handler) ResolveAdminReport(ctx context.Context, request api.ResolveAdminReportRequestObject) (api.ResolveAdminReportResponseObject, error) {
	admin, err := accounts.RequireInstanceAdmin(ctx, handler.users)
	if errors.Is(err, accounts.ErrForbidden) {
		return api.ResolveAdminReport403JSONResponse{ForbiddenJSONResponse: forbidden(err)}, nil
	}
	if err != nil {
		return nil, err
	}
	err = handler.service.ResolveAdminReport(ctx, admin.ID, request.ReportId, resolution(request.Body))
	if known, ok := classify(err); ok {
		body := apiresponse.Error(known.reason, err)
		switch known.code {
		case 404:
			return api.ResolveAdminReport404JSONResponse(body), nil
		case 409:
			return api.ResolveAdminReport409JSONResponse(body), nil
		}
		return api.ResolveAdminReport422JSONResponse(body), nil
	}
	if err != nil {
		return nil, err
	}
	return api.ResolveAdminReport204Response{}, nil
}
