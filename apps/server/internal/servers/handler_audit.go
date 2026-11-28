package servers

import (
	"context"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// defaultAuditPage is how many entries a page has unless the caller asks.
const defaultAuditPage = 50

// ListServerAuditLog answers GET /api/v1/servers/{serverId}/audit-log.
func (handler *Handler) ListServerAuditLog(ctx context.Context, request api.ListServerAuditLogRequestObject) (api.ListServerAuditLogResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	entries, err := handler.service.AuditLog(ctx, where, auditFilter(request.Params))
	if out, refused := refusal[api.ListServerAuditLogResponseObject, api.ListServerAuditLog403JSONResponse, api.ListServerAuditLog404JSONResponse, api.ListServerAuditLog404JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListServerAuditLog200JSONResponse, 0, len(entries))
	for _, entry := range entries {
		out = append(out, auditResponse(entry))
	}
	return out, nil
}

func auditFilter(params api.ListServerAuditLogParams) AuditFilter {
	filter := AuditFilter{ActorID: params.ActorId, Limit: defaultAuditPage}
	if params.Before != nil {
		filter.Before = *params.Before
	}
	if params.Action != nil {
		filter.Action = *params.Action
	}
	if params.TargetId != nil {
		filter.TargetID = *params.TargetId
	}
	if params.Limit != nil {
		filter.Limit = *params.Limit
	}
	return filter
}

func auditResponse(entry AuditEntry) api.AuditEntry {
	out := api.AuditEntry{Id: entry.ID, ActorId: entry.ActorID, Action: entry.Action, Details: entry.Details, CreatedAt: entry.CreatedAt}
	if entry.TargetKind != "" {
		kind, id := api.AuditEntryTargetKind(entry.TargetKind), entry.TargetID
		out.TargetKind, out.TargetId = &kind, &id
	}
	return out
}
