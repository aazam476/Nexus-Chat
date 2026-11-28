package reports

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// errNoPrincipal means a protected handler ran without authentication,
// which the router prevents.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// Handler serves the report endpoints.
type Handler struct {
	service *Service
	users   accounts.UserReader
}

// NewHandler returns a Handler; users is used to check instance admins.
func NewHandler(service *Service, users accounts.UserReader) *Handler {
	return &Handler{service: service, users: users}
}

func caller(ctx context.Context) (uuid.UUID, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return uuid.Nil, errNoPrincipal
	}
	return principal.UserID, nil
}

// SubmitReport answers POST /api/v1/reports.
func (handler *Handler) SubmitReport(ctx context.Context, request api.SubmitReportRequestObject) (api.SubmitReportResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	body := request.Body
	report := NewReport{
		Target:   Target{ServerID: body.ServerId, ChannelID: body.ChannelId, ConversationID: body.ConversationId, MessageID: body.MessageId},
		Evidence: body.Evidence,
	}
	if body.Comment != nil {
		report.Comment = *body.Comment
	}
	id, err := handler.service.Submit(ctx, me, report)
	switch {
	case errors.Is(err, ErrNotFound):
		return api.SubmitReport404JSONResponse(apiresponse.Error("message_not_found", err)), nil
	case errors.Is(err, ErrReported):
		return api.SubmitReport409JSONResponse(apiresponse.Error("already_reported", err)), nil
	case errors.Is(err, ErrOwn):
		return api.SubmitReport422JSONResponse(apiresponse.Error("own_message", err)), nil
	case errors.Is(err, ErrBadEvidence):
		return api.SubmitReport422JSONResponse(apiresponse.Error("bad_evidence", err)), nil
	case errors.Is(err, ErrInvalid):
		return api.SubmitReport422JSONResponse(apiresponse.Error("invalid", err)), nil
	case err != nil:
		return nil, err
	}
	return api.SubmitReport201JSONResponse{Id: id}, nil
}

func statusOf[S ~string](status *S) string {
	if status == nil {
		return StatusOpen
	}
	return string(*status)
}

func reportResponses(list []Report) []api.Report {
	out := make([]api.Report, 0, len(list))
	for _, report := range list {
		out = append(out, api.Report{
			Id: report.ID, ServerId: report.ServerID, ChannelId: report.ChannelID, ConversationId: report.ConversationID,
			MessageId: report.MessageID, ReporterId: report.ReporterID, SenderId: report.SenderID, Evidence: report.Evidence,
			Receipt: report.Receipt, ReceiptSignature: report.ReceiptSig, Comment: report.Comment, Status: api.ReportStatus(report.Status),
			Escalated: report.Escalated, Resolution: report.Resolution, ResolvedBy: report.ResolvedBy, ResolvedAt: report.ResolvedAt,
			CreatedAt: report.CreatedAt,
		})
	}
	return out
}

// ListServerReports answers GET /api/v1/servers/{serverId}/reports.
func (handler *Handler) ListServerReports(ctx context.Context, request api.ListServerReportsRequestObject) (api.ListServerReportsResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	list, err := handler.service.ServerReports(ctx, request.ServerId, me, statusOf(request.Params.Status))
	switch {
	case errors.Is(err, ErrForbidden):
		return api.ListServerReports403JSONResponse(apiresponse.Error("missing_permission", err)), nil
	case errors.Is(err, ErrNoServer):
		return api.ListServerReports404JSONResponse(apiresponse.Error("server_not_found", err)), nil
	case err != nil:
		return nil, err
	}
	return api.ListServerReports200JSONResponse(reportResponses(list)), nil
}

// actionError is the API's answer to a failed resolve or escalate: 403, 404, 409 or 422.
type actionError struct {
	code   int
	reason string
}

func classify(err error) (actionError, bool) {
	switch {
	case errors.Is(err, ErrForbidden):
		return actionError{403, "missing_permission"}, true
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrNoServer):
		return actionError{404, "not_found"}, true
	case errors.Is(err, ErrHandled):
		return actionError{409, "already_handled"}, true
	case errors.Is(err, ErrInvalid):
		return actionError{422, "invalid"}, true
	}
	return actionError{}, false
}

func resolution(body *api.ReportResolution) string {
	if body == nil || body.Resolution == nil {
		return ""
	}
	return *body.Resolution
}

// ResolveServerReport answers POST /api/v1/servers/{serverId}/reports/{reportId}/resolve.
func (handler *Handler) ResolveServerReport(ctx context.Context, request api.ResolveServerReportRequestObject) (api.ResolveServerReportResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.ResolveServerReport(ctx, Action{ServerID: request.ServerId, UserID: me, ReportID: request.ReportId, Resolution: resolution(request.Body)})
	if known, ok := classify(err); ok {
		body := apiresponse.Error(known.reason, err)
		switch known.code {
		case 403:
			return api.ResolveServerReport403JSONResponse(body), nil
		case 404:
			return api.ResolveServerReport404JSONResponse(body), nil
		case 409:
			return api.ResolveServerReport409JSONResponse(body), nil
		}
		return api.ResolveServerReport422JSONResponse(body), nil
	}
	if err != nil {
		return nil, err
	}
	return api.ResolveServerReport204Response{}, nil
}

// EscalateServerReport answers POST /api/v1/servers/{serverId}/reports/{reportId}/escalate.
func (handler *Handler) EscalateServerReport(ctx context.Context, request api.EscalateServerReportRequestObject) (api.EscalateServerReportResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.EscalateServerReport(ctx, Action{ServerID: request.ServerId, UserID: me, ReportID: request.ReportId})
	if known, ok := classify(err); ok {
		body := apiresponse.Error(known.reason, err)
		switch known.code {
		case 403:
			return api.EscalateServerReport403JSONResponse(body), nil
		case 404:
			return api.EscalateServerReport404JSONResponse(body), nil
		}
		return api.EscalateServerReport409JSONResponse(body), nil
	}
	if err != nil {
		return nil, err
	}
	return api.EscalateServerReport204Response{}, nil
}
