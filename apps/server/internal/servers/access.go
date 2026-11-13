package servers

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// access is everything about one member that permission checks need,
// loaded once per request.
type access struct {
	server    store.Server
	userID    uuid.UUID
	timedOut  bool
	roles     []store.ServerRole
	myRoles   map[uuid.UUID]bool
	overrides []store.ServerChannelOverride
}

// load reads the member's access to the server, or ErrNotFound when they
// are not a member (so a server's existence is not revealed).
func (service *Service) load(ctx context.Context, scope Scope) (access, error) {
	serverID, userID := scope.ServerID, scope.UserID
	member, err := service.store.GetServerMember(ctx, store.GetServerMemberParams{ServerID: serverID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return access{}, ErrNotFound
	}
	if err != nil {
		return access{}, err
	}
	server, err := service.store.GetServer(ctx, serverID)
	if err != nil {
		return access{}, err
	}
	roles, err := service.store.ListRoles(ctx, serverID)
	if err != nil {
		return access{}, err
	}
	mine, err := service.store.MemberRoleIDs(ctx, store.MemberRoleIDsParams{ServerID: serverID, UserID: userID})
	if err != nil {
		return access{}, err
	}
	overrides, err := service.store.ListOverrides(ctx, serverID)
	if err != nil {
		return access{}, err
	}
	timedOut := member.TimedOutUntil != nil && member.TimedOutUntil.After(service.clock.Now())
	myRoles := map[uuid.UUID]bool{}
	for _, id := range mine {
		myRoles[id] = true
	}
	return access{server: server, userID: userID, timedOut: timedOut, roles: roles, myRoles: myRoles, overrides: overrides}, nil
}

func (a access) owner() bool {
	return a.server.OwnerID == a.userID
}

// member is the input to internal/permissions for this member.
func (a access) member() permissions.Member {
	member := permissions.Member{Owner: a.owner(), TimedOut: a.timedOut}
	for _, role := range a.roles {
		switch {
		case role.Everyone:
			member.Everyone = permissions.Permission(role.Permissions) //nolint:gosec // stored non-negative
		case a.myRoles[role.ID]:
			member.Roles = append(member.Roles, permissions.Permission(role.Permissions)) //nolint:gosec // stored non-negative
		}
	}
	return member
}

// inServer is what the member may do outside any channel.
func (a access) inServer() permissions.Permission {
	return permissions.Server(a.member())
}

// inChannel is what the member may do in the channel.
func (a access) inChannel(channelID uuid.UUID) permissions.Permission {
	var channel permissions.Channel
	everyone := a.everyoneRole()
	for _, o := range a.overrides {
		if o.ChannelID != channelID {
			continue
		}
		override := permissions.Override{Allow: permissions.Permission(o.AllowBits), Deny: permissions.Permission(o.DenyBits)} //nolint:gosec // stored non-negative
		switch {
		case o.TargetKind == "role" && o.TargetID == everyone:
			channel.Everyone = override
		case o.TargetKind == "role" && a.myRoles[o.TargetID]:
			channel.Roles = append(channel.Roles, override)
		case o.TargetKind == "member" && o.TargetID == a.userID:
			channel.Member = override
		}
	}
	return permissions.InChannel(a.member(), channel)
}

func (a access) everyoneRole() uuid.UUID {
	for _, role := range a.roles {
		if role.Everyone {
			return role.ID
		}
	}
	return uuid.Nil
}

// top is the position of the member's highest role; the owner outranks everyone.
func (a access) top() int {
	if a.owner() {
		return math.MaxInt32
	}
	top := 0
	for _, role := range a.roles {
		if a.myRoles[role.ID] && int(role.Position) > top {
			top = int(role.Position)
		}
	}
	return top
}

// require fails with ErrForbidden unless the member has want in the server.
func (a access) require(want permissions.Permission) error {
	if !a.inServer().Has(want) {
		return ErrForbidden
	}
	return nil
}

// mayGrant reports whether the member may hand out every permission in p:
// only those they have themselves, unless they are an administrator.
func (a access) mayGrant(p permissions.Permission) bool {
	have := a.inServer()
	return have.Has(permissions.Administrator) || have.Has(p)
}

// role finds one of the server's roles.
func (a access) role(id uuid.UUID) (store.ServerRole, bool) {
	for _, role := range a.roles {
		if role.ID == id {
			return role, true
		}
	}
	return store.ServerRole{}, false
}
