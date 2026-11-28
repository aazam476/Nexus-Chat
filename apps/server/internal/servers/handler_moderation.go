package servers

import (
	"context"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// KickServerMember answers POST /api/v1/servers/{serverId}/members/{userId}/kick.
func (handler *Handler) KickServerMember(ctx context.Context, request api.KickServerMemberRequestObject) (api.KickServerMemberResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.Kick(ctx, where, Sanction{UserID: request.UserId, Reason: text(request.Body.Reason)})
	if out, refused := refusal[api.KickServerMemberResponseObject, api.KickServerMember403JSONResponse, api.KickServerMember404JSONResponse, api.KickServerMember422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.KickServerMember204Response{}, nil
}

// TimeoutServerMember answers PUT /api/v1/servers/{serverId}/members/{userId}/timeout.
func (handler *Handler) TimeoutServerMember(ctx context.Context, request api.TimeoutServerMemberRequestObject) (api.TimeoutServerMemberResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	length := time.Duration(request.Body.Seconds) * time.Second
	if length == 0 {
		// Zero would lift a timeout; this operation only sets one.
		length = -1
	}
	err = handler.service.Timeout(ctx, where, request.UserId, length)
	if out, refused := refusal[api.TimeoutServerMemberResponseObject, api.TimeoutServerMember403JSONResponse, api.TimeoutServerMember404JSONResponse, api.TimeoutServerMember422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.TimeoutServerMember204Response{}, nil
}

// EndServerMemberTimeout answers DELETE /api/v1/servers/{serverId}/members/{userId}/timeout.
func (handler *Handler) EndServerMemberTimeout(ctx context.Context, request api.EndServerMemberTimeoutRequestObject) (api.EndServerMemberTimeoutResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.Timeout(ctx, where, request.UserId, 0)
	if out, refused := refusal[api.EndServerMemberTimeoutResponseObject, api.EndServerMemberTimeout403JSONResponse, api.EndServerMemberTimeout404JSONResponse, api.EndServerMemberTimeout404JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.EndServerMemberTimeout204Response{}, nil
}

// ListServerBans answers GET /api/v1/servers/{serverId}/bans.
func (handler *Handler) ListServerBans(ctx context.Context, request api.ListServerBansRequestObject) (api.ListServerBansResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	bans, err := handler.service.Bans(ctx, where)
	if out, refused := refusal[api.ListServerBansResponseObject, api.ListServerBans403JSONResponse, api.ListServerBans404JSONResponse, api.ListServerBans404JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(api.ListServerBans200JSONResponse, 0, len(bans))
	for _, ban := range bans {
		entry := api.ServerBan{BannedBy: ban.BannedBy, Reason: ban.Reason, CreatedAt: ban.CreatedAt}
		entry.User.Id, entry.User.Username, entry.User.DisplayName = ban.UserID, ban.Username, ban.DisplayName
		out = append(out, entry)
	}
	return out, nil
}

// BanServerMember answers PUT /api/v1/servers/{serverId}/bans/{userId}.
func (handler *Handler) BanServerMember(ctx context.Context, request api.BanServerMemberRequestObject) (api.BanServerMemberResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	ban := BanRequest{UserID: request.UserId, Reason: text(request.Body.Reason)}
	if request.Body.DeleteMessageSeconds != nil {
		ban.DeleteMessages = time.Duration(*request.Body.DeleteMessageSeconds) * time.Second
	}
	err = handler.service.Ban(ctx, where, ban)
	if out, refused := refusal[api.BanServerMemberResponseObject, api.BanServerMember403JSONResponse, api.BanServerMember404JSONResponse, api.BanServerMember422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.BanServerMember204Response{}, nil
}

// UnbanServerMember answers DELETE /api/v1/servers/{serverId}/bans/{userId}.
func (handler *Handler) UnbanServerMember(ctx context.Context, request api.UnbanServerMemberRequestObject) (api.UnbanServerMemberResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.Unban(ctx, where, request.UserId)
	if out, refused := refusal[api.UnbanServerMemberResponseObject, api.UnbanServerMember403JSONResponse, api.UnbanServerMember404JSONResponse, api.UnbanServerMember404JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.UnbanServerMember204Response{}, nil
}

// text is an optional string, empty when absent.
func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
