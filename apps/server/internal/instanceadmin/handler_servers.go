package instanceadmin

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers"
)

// ListAdminServers answers GET /api/v1/admin/servers.
func (handler *Handler) ListAdminServers(ctx context.Context, request api.ListAdminServersRequestObject) (api.ListAdminServersResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.ListAdminServers403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	after := ServerCursor{Name: text(request.Params.AfterName), ID: text(request.Params.AfterId)}
	list, err := handler.service.Servers(ctx, text(request.Params.Q), after)
	if errors.Is(err, ErrInvalid) {
		return api.ListAdminServers422JSONResponse(apiresponse.Error("invalid", err)), nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListAdminServers200JSONResponse, 0, len(list))
	for _, server := range list {
		out = append(out, api.AdminServer{
			Id: server.ID, Name: server.Name, OwnerId: server.OwnerID, OwnerUsername: server.OwnerUsername, Members: server.Members, CreatedAt: server.CreatedAt,
		})
	}
	return out, nil
}

// DeleteAdminServer answers DELETE /api/v1/admin/servers/{serverId}.
func (handler *Handler) DeleteAdminServer(ctx context.Context, request api.DeleteAdminServerRequestObject) (api.DeleteAdminServerResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.DeleteAdminServer403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	err = handler.service.DeleteServer(ctx, request.ServerId)
	if errors.Is(err, servers.ErrNotFound) {
		return api.DeleteAdminServer404JSONResponse(apiresponse.Error("server_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.DeleteAdminServer204Response{}, nil
}

// GetInstanceStats answers GET /api/v1/admin/stats.
func (handler *Handler) GetInstanceStats(ctx context.Context, _ api.GetInstanceStatsRequestObject) (api.GetInstanceStatsResponseObject, error) {
	_, forbidden, err := handler.admin(ctx)
	if err != nil {
		return nil, err
	}
	if forbidden != nil {
		return api.GetInstanceStats403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	stats, err := handler.service.Stats(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetInstanceStats200JSONResponse{
		Users: stats.Users, SuspendedUsers: stats.SuspendedUsers, ActiveUsers: stats.ActiveUsers, Servers: stats.Servers,
		Conversations: stats.Conversations, RecentMessages: stats.RecentMessages, OpenReports: stats.OpenReports, AttachmentBytes: stats.AttachmentBytes,
	}, nil
}
