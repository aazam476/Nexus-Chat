package servers

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// ListServers answers GET /api/v1/servers.
func (handler *Handler) ListServers(ctx context.Context, _ api.ListServersRequestObject) (api.ListServersResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	list, err := handler.service.List(ctx, me)
	if err != nil {
		return nil, err
	}
	out := make(api.ListServers200JSONResponse, 0, len(list))
	for _, server := range list {
		out = append(out, serverResponse(server))
	}
	return out, nil
}

// CreateServer answers POST /api/v1/servers.
func (handler *Handler) CreateServer(ctx context.Context, request api.CreateServerRequestObject) (api.CreateServerResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	description := ""
	if request.Body.Description != nil {
		description = *request.Body.Description
	}
	details, err := handler.service.Create(ctx, me, request.Body.Name, description)
	if errors.Is(err, ErrInvalid) {
		return api.CreateServer422JSONResponse(apiresponse.Error("invalid", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.CreateServer201JSONResponse(detailsResponse(details)), nil
}

// GetServer answers GET /api/v1/servers/{serverId}.
func (handler *Handler) GetServer(ctx context.Context, request api.GetServerRequestObject) (api.GetServerResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	details, err := handler.service.Details(ctx, where)
	if out, refused := refusal[api.GetServerResponseObject, api.GetServer403JSONResponse, api.GetServer404JSONResponse, api.GetServer422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetServer200JSONResponse(detailsResponse(details)), nil
}

// UpdateServer answers PATCH /api/v1/servers/{serverId}.
func (handler *Handler) UpdateServer(ctx context.Context, request api.UpdateServerRequestObject) (api.UpdateServerResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	body := request.Body
	err = handler.service.Update(ctx, where, Settings{
		Name: body.Name, Description: body.Description, AttachmentMaxBytes: body.AttachmentMaxBytes, CompressMedia: body.CompressMedia,
	})
	if out, refused := refusal[api.UpdateServerResponseObject, api.UpdateServer403JSONResponse, api.UpdateServer404JSONResponse, api.UpdateServer422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateServer204Response{}, nil
}

// DeleteServer answers DELETE /api/v1/servers/{serverId}.
func (handler *Handler) DeleteServer(ctx context.Context, request api.DeleteServerRequestObject) (api.DeleteServerResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.Delete(ctx, where)
	if out, refused := refusal[api.DeleteServerResponseObject, api.DeleteServer403JSONResponse, api.DeleteServer404JSONResponse, api.DeleteServer422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.DeleteServer204Response{}, nil
}

// LeaveServer answers DELETE /api/v1/servers/{serverId}/members/me.
func (handler *Handler) LeaveServer(ctx context.Context, request api.LeaveServerRequestObject) (api.LeaveServerResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.Leave(ctx, where)
	if out, refused := refusal[api.LeaveServerResponseObject, api.LeaveServer403JSONResponse, api.LeaveServer404JSONResponse, api.LeaveServer422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.LeaveServer204Response{}, nil
}

// ListServerMembers answers GET /api/v1/servers/{serverId}/members.
func (handler *Handler) ListServerMembers(ctx context.Context, request api.ListServerMembersRequestObject) (api.ListServerMembersResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	members, err := handler.service.Members(ctx, where)
	if out, refused := refusal[api.ListServerMembersResponseObject, api.ListServerMembers403JSONResponse, api.ListServerMembers404JSONResponse, api.ListServerMembers422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListServerMembers200JSONResponse, 0, len(members))
	for _, member := range members {
		out = append(out, memberResponse(member))
	}
	return out, nil
}

// GiveServerRole answers PUT /api/v1/servers/{serverId}/members/{userId}/roles/{roleId}.
func (handler *Handler) GiveServerRole(ctx context.Context, request api.GiveServerRoleRequestObject) (api.GiveServerRoleResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.SetMemberRole(ctx, where, RoleChange{UserID: request.UserId, RoleID: request.RoleId, Give: true})
	if out, refused := refusal[api.GiveServerRoleResponseObject, api.GiveServerRole403JSONResponse, api.GiveServerRole404JSONResponse, api.GiveServerRole422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.GiveServerRole204Response{}, nil
}

// TakeServerRole answers DELETE /api/v1/servers/{serverId}/members/{userId}/roles/{roleId}.
func (handler *Handler) TakeServerRole(ctx context.Context, request api.TakeServerRoleRequestObject) (api.TakeServerRoleResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.SetMemberRole(ctx, where, RoleChange{UserID: request.UserId, RoleID: request.RoleId})
	if out, refused := refusal[api.TakeServerRoleResponseObject, api.TakeServerRole403JSONResponse, api.TakeServerRole404JSONResponse, api.TakeServerRole422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.TakeServerRole204Response{}, nil
}
