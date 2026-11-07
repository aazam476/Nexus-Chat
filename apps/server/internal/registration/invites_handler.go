package registration

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

var errInviteNotFound = errors.New("there is no invite with this ID")

// CreateInvite answers POST /api/v1/admin/invites.
func (handler *Handler) CreateInvite(ctx context.Context, request api.CreateInviteRequestObject) (api.CreateInviteResponseObject, error) {
	admin, err := accounts.RequireInstanceAdmin(ctx, handler.users)
	if errors.Is(err, accounts.ErrForbidden) {
		return api.CreateInvite403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(apiresponse.Error("forbidden", err))}, nil
	}
	if err != nil {
		return nil, err
	}
	options := InviteOptions{ExpiresAt: request.Body.ExpiresAt}
	if request.Body.MaxUses != nil {
		options.MaxUses = *request.Body.MaxUses
	}
	code, id, err := handler.invites.Create(ctx, admin.ID, options)
	if err != nil {
		return nil, err
	}
	return api.CreateInvite201JSONResponse{Id: id, Code: code}, nil
}

// ListInvites answers GET /api/v1/admin/invites.
func (handler *Handler) ListInvites(ctx context.Context, _ api.ListInvitesRequestObject) (api.ListInvitesResponseObject, error) {
	_, err := accounts.RequireInstanceAdmin(ctx, handler.users)
	if errors.Is(err, accounts.ErrForbidden) {
		return api.ListInvites403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(apiresponse.Error("forbidden", err))}, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := handler.invites.List(ctx)
	if err != nil {
		return nil, err
	}
	response := make(api.ListInvites200JSONResponse, 0, len(rows))
	for _, row := range rows {
		response = append(response, api.Invite{Id: row.ID, MaxUses: row.MaxUses, Uses: row.Uses, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt})
	}
	return response, nil
}

// DeleteInvite answers DELETE /api/v1/admin/invites/{inviteId}.
func (handler *Handler) DeleteInvite(ctx context.Context, request api.DeleteInviteRequestObject) (api.DeleteInviteResponseObject, error) {
	_, err := accounts.RequireInstanceAdmin(ctx, handler.users)
	if errors.Is(err, accounts.ErrForbidden) {
		return api.DeleteInvite403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(apiresponse.Error("forbidden", err))}, nil
	}
	if err != nil {
		return nil, err
	}
	deleted, err := handler.invites.Delete(ctx, request.InviteId)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return api.DeleteInvite404JSONResponse(apiresponse.Error("invite_not_found", errInviteNotFound)), nil
	}
	return api.DeleteInvite204Response{}, nil
}
