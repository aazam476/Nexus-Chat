package servers

import (
	"context"
	"errors"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// ListServerInvites answers GET /api/v1/servers/{serverId}/invites.
func (handler *Handler) ListServerInvites(ctx context.Context, request api.ListServerInvitesRequestObject) (api.ListServerInvitesResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	invites, err := handler.service.Invites(ctx, where)
	if out, refused := refusal[api.ListServerInvitesResponseObject, api.ListServerInvites403JSONResponse, api.ListServerInvites404JSONResponse, api.ListServerInvites422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListServerInvites200JSONResponse, 0, len(invites))
	for _, invite := range invites {
		out = append(out, inviteResponse(invite))
	}
	return out, nil
}

// CreateServerInvite answers POST /api/v1/servers/{serverId}/invites.
func (handler *Handler) CreateServerInvite(ctx context.Context, request api.CreateServerInviteRequestObject) (api.CreateServerInviteResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	limits := InviteLimits{MaxUses: request.Body.MaxUses}
	if seconds := request.Body.ExpiresInSeconds; seconds != nil {
		// Out-of-range values are refused by the service; clamp first so
		// the multiplication cannot overflow into a valid duration.
		lifetime := time.Duration(min(max(*seconds, -1), int64(maximumInviteLifetime/time.Second)+1)) * time.Second
		limits.ExpiresIn = &lifetime
	}
	invite, err := handler.service.CreateInvite(ctx, where, limits)
	if out, refused := refusal[api.CreateServerInviteResponseObject, api.CreateServerInvite403JSONResponse, api.CreateServerInvite404JSONResponse, api.CreateServerInvite422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.CreateServerInvite201JSONResponse(inviteResponse(invite)), nil
}

// RevokeServerInvite answers DELETE /api/v1/servers/{serverId}/invites/{code}.
func (handler *Handler) RevokeServerInvite(ctx context.Context, request api.RevokeServerInviteRequestObject) (api.RevokeServerInviteResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.RevokeInvite(ctx, where, request.Code)
	if out, refused := refusal[api.RevokeServerInviteResponseObject, api.RevokeServerInvite403JSONResponse, api.RevokeServerInvite404JSONResponse, api.RevokeServerInvite422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.RevokeServerInvite204Response{}, nil
}

// PreviewInvite answers GET /api/v1/invites/{code}.
func (handler *Handler) PreviewInvite(ctx context.Context, request api.PreviewInviteRequestObject) (api.PreviewInviteResponseObject, error) {
	preview, err := handler.service.PreviewInvite(ctx, request.Code)
	if errors.Is(err, ErrInviteInvalid) {
		return api.PreviewInvite404JSONResponse(apiresponse.Error("invite_invalid", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.PreviewInvite200JSONResponse{Server: serverResponse(preview.Server), Members: preview.Members}, nil
}

// JoinServer answers POST /api/v1/invites/{code}/join.
func (handler *Handler) JoinServer(ctx context.Context, request api.JoinServerRequestObject) (api.JoinServerResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	serverID, err := handler.service.Join(ctx, request.Code, me)
	if errors.Is(err, ErrInviteInvalid) {
		return api.JoinServer404JSONResponse(apiresponse.Error("invite_invalid", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.JoinServer200JSONResponse{ServerId: serverID}, nil
}
