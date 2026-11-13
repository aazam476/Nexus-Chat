package servers

import (
	"context"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// roleSpec reads a role from the API.
func roleSpec(body api.RoleSpec) (RoleSpec, error) {
	granted, err := parsePermissions(body.Permissions)
	return RoleSpec{Name: body.Name, Color: body.Color, Permissions: granted}, err
}

// CreateServerRole answers POST /api/v1/servers/{serverId}/roles.
func (handler *Handler) CreateServerRole(ctx context.Context, request api.CreateServerRoleRequestObject) (api.CreateServerRoleResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	id, err := handler.createRole(ctx, where, *request.Body)
	if out, refused := refusal[api.CreateServerRoleResponseObject, api.CreateServerRole403JSONResponse, api.CreateServerRole404JSONResponse, api.CreateServerRole422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.CreateServerRole201JSONResponse{Id: id}, nil
}

func (handler *Handler) createRole(ctx context.Context, where Scope, body api.RoleSpec) (uuid.UUID, error) {
	spec, err := roleSpec(body)
	if err != nil {
		return uuid.Nil, err
	}
	return handler.service.CreateRole(ctx, where, spec)
}

// UpdateServerRole answers PATCH /api/v1/servers/{serverId}/roles/{roleId}.
func (handler *Handler) UpdateServerRole(ctx context.Context, request api.UpdateServerRoleRequestObject) (api.UpdateServerRoleResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	spec, err := roleSpec(*request.Body)
	if err == nil {
		err = handler.service.UpdateRole(ctx, where, request.RoleId, spec)
	}
	if out, refused := refusal[api.UpdateServerRoleResponseObject, api.UpdateServerRole403JSONResponse, api.UpdateServerRole404JSONResponse, api.UpdateServerRole422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateServerRole204Response{}, nil
}

// DeleteServerRole answers DELETE /api/v1/servers/{serverId}/roles/{roleId}.
func (handler *Handler) DeleteServerRole(ctx context.Context, request api.DeleteServerRoleRequestObject) (api.DeleteServerRoleResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.DeleteRole(ctx, where, request.RoleId)
	if out, refused := refusal[api.DeleteServerRoleResponseObject, api.DeleteServerRole403JSONResponse, api.DeleteServerRole404JSONResponse, api.DeleteServerRole422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.DeleteServerRole204Response{}, nil
}

// OrderServerRoles answers PUT /api/v1/servers/{serverId}/role-order.
func (handler *Handler) OrderServerRoles(ctx context.Context, request api.OrderServerRolesRequestObject) (api.OrderServerRolesResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.OrderRoles(ctx, where, request.Body.RoleIds)
	if out, refused := refusal[api.OrderServerRolesResponseObject, api.OrderServerRoles403JSONResponse, api.OrderServerRoles404JSONResponse, api.OrderServerRoles422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.OrderServerRoles204Response{}, nil
}
