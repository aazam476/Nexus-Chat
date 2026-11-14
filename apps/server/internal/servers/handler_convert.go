package servers

import (
	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

func serverResponse(server Server) api.Server {
	return api.Server{
		Id: server.ID, Name: server.Name, Description: server.Description, OwnerId: server.OwnerID,
		AttachmentMaxBytes: server.AttachmentMaxBytes, CompressMedia: server.CompressMedia, CreatedAt: server.CreatedAt,
	}
}

func roleResponse(role Role) api.ServerRole {
	return api.ServerRole{
		Id: role.ID, Name: role.Name, Color: role.Color, Permissions: permissionNames(role.Permissions),
		Position: role.Position, Everyone: role.Everyone,
	}
}

func channelResponse(channel Channel) api.ServerChannel {
	overrides := make([]api.ChannelOverride, 0, len(channel.Overrides))
	for _, override := range channel.Overrides {
		overrides = append(overrides, api.ChannelOverride{
			TargetKind: api.OverrideTargetKind(override.TargetKind), TargetId: override.TargetID,
			Allow: permissionNames(override.Allow), Deny: permissionNames(override.Deny),
		})
	}
	out := api.ServerChannel{
		Id: channel.ID, Kind: api.ServerChannelKind(channel.Kind), Name: channel.Name, Topic: channel.Topic,
		ParentId: channel.ParentID, Position: channel.Position, Overrides: overrides, AudienceId: channel.AudienceID,
		Permissions: permissionNames(channel.Permissions),
	}
	if channel.MLSGroupID != nil {
		out.MlsGroupId = &channel.MLSGroupID
	}
	return out
}

func detailsResponse(details Details) api.ServerDetails {
	roles := make([]api.ServerRole, 0, len(details.Roles))
	for _, role := range details.Roles {
		roles = append(roles, roleResponse(role))
	}
	channels := make([]api.ServerChannel, 0, len(details.Channels))
	for _, channel := range details.Channels {
		channels = append(channels, channelResponse(channel))
	}
	return api.ServerDetails{
		Server: serverResponse(details.Server), Roles: roles, Channels: channels,
		MyRoleIds: nonNil(details.MyRoleIDs), Permissions: permissionNames(details.Permissions),
	}
}

func memberResponse(member Member) api.ServerMember {
	return api.ServerMember{
		User: api.UserSummary{
			Id: member.UserID, Username: member.Username, DisplayName: member.DisplayName,
			AvatarUrl: apiresponse.AvatarPath(member.AvatarID),
		},
		JoinedAt: member.JoinedAt, RoleIds: nonNil(member.RoleIDs),
	}
}

func inviteResponse(invite Invite) api.ServerInvite {
	return api.ServerInvite{
		Code: invite.Code, ServerId: invite.ServerID, CreatedBy: invite.CreatedBy, MaxUses: invite.MaxUses,
		Uses: invite.Uses, ExpiresAt: invite.ExpiresAt, CreatedAt: invite.CreatedAt,
	}
}

// nonNil makes an empty list encode as [] rather than null.
func nonNil(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}
