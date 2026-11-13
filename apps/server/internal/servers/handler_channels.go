package servers

import (
	"context"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// CreateServerChannel answers POST /api/v1/servers/{serverId}/channels.
func (handler *Handler) CreateServerChannel(ctx context.Context, request api.CreateServerChannelRequestObject) (api.CreateServerChannelResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	body := request.Body
	spec := NewChannel{Kind: string(body.Kind), Name: body.Name, ParentID: body.ParentId}
	if body.Topic != nil {
		spec.Topic = *body.Topic
	}
	id, err := handler.service.CreateChannel(ctx, where, spec)
	if out, refused := refusal[api.CreateServerChannelResponseObject, api.CreateServerChannel403JSONResponse, api.CreateServerChannel404JSONResponse, api.CreateServerChannel422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.CreateServerChannel201JSONResponse{Id: id}, nil
}

// UpdateServerChannel answers PATCH /api/v1/servers/{serverId}/channels/{channelId}.
func (handler *Handler) UpdateServerChannel(ctx context.Context, request api.UpdateServerChannelRequestObject) (api.UpdateServerChannelResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	text := ChannelText{Name: request.Body.Name}
	if request.Body.Topic != nil {
		text.Topic = *request.Body.Topic
	}
	err = handler.service.UpdateChannel(ctx, where, request.ChannelId, text)
	if out, refused := refusal[api.UpdateServerChannelResponseObject, api.UpdateServerChannel403JSONResponse, api.UpdateServerChannel404JSONResponse, api.UpdateServerChannel422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateServerChannel204Response{}, nil
}

// DeleteServerChannel answers DELETE /api/v1/servers/{serverId}/channels/{channelId}.
func (handler *Handler) DeleteServerChannel(ctx context.Context, request api.DeleteServerChannelRequestObject) (api.DeleteServerChannelResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.DeleteChannel(ctx, where, request.ChannelId)
	if out, refused := refusal[api.DeleteServerChannelResponseObject, api.DeleteServerChannel403JSONResponse, api.DeleteServerChannel404JSONResponse, api.DeleteServerChannel422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.DeleteServerChannel204Response{}, nil
}

// OrderServerChannels answers PUT /api/v1/servers/{serverId}/channel-order.
func (handler *Handler) OrderServerChannels(ctx context.Context, request api.OrderServerChannelsRequestObject) (api.OrderServerChannelsResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	order := make([]Placement, 0, len(*request.Body))
	for _, placement := range *request.Body {
		order = append(order, Placement{ID: placement.Id, ParentID: placement.ParentId})
	}
	err = handler.service.OrderChannels(ctx, where, order)
	if out, refused := refusal[api.OrderServerChannelsResponseObject, api.OrderServerChannels403JSONResponse, api.OrderServerChannels404JSONResponse, api.OrderServerChannels422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.OrderServerChannels204Response{}, nil
}

// SetChannelOverride answers PUT /api/v1/servers/{serverId}/channels/{channelId}/overrides/{targetKind}/{targetId}.
func (handler *Handler) SetChannelOverride(ctx context.Context, request api.SetChannelOverrideRequestObject) (api.SetChannelOverrideResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.setOverride(ctx, where, request)
	if out, refused := refusal[api.SetChannelOverrideResponseObject, api.SetChannelOverride403JSONResponse, api.SetChannelOverride404JSONResponse, api.SetChannelOverride422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.SetChannelOverride204Response{}, nil
}

func (handler *Handler) setOverride(ctx context.Context, where Scope, request api.SetChannelOverrideRequestObject) error {
	allow, err := parsePermissions(request.Body.Allow)
	if err != nil {
		return err
	}
	deny, err := parsePermissions(request.Body.Deny)
	if err != nil {
		return err
	}
	override := Override{TargetKind: string(request.TargetKind), TargetID: request.TargetId, Allow: allow, Deny: deny}
	return handler.service.SetOverride(ctx, where, request.ChannelId, override)
}

// ClearChannelOverride answers DELETE /api/v1/servers/{serverId}/channels/{channelId}/overrides/{targetKind}/{targetId}.
func (handler *Handler) ClearChannelOverride(ctx context.Context, request api.ClearChannelOverrideRequestObject) (api.ClearChannelOverrideResponseObject, error) {
	where, err := scope(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	err = handler.service.ClearOverride(ctx, where, request.ChannelId, Target{Kind: string(request.TargetKind), ID: request.TargetId})
	if out, refused := refusal[api.ClearChannelOverrideResponseObject, api.ClearChannelOverride403JSONResponse, api.ClearChannelOverride404JSONResponse, api.ClearChannelOverride422JSONResponse](err); refused {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return api.ClearChannelOverride204Response{}, nil
}
