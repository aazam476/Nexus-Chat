package servers

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// RoleSpec is a role's name, color and permissions.
type RoleSpec struct {
	Name        string
	Color       *int32
	Permissions permissions.Permission
}

// managingRoles loads the member's access and requires MANAGE_ROLES.
func (service *Service) managingRoles(ctx context.Context, scope Scope) (access, error) {
	a, err := service.load(ctx, scope)
	if err != nil {
		return access{}, err
	}
	return a, a.require(permissions.ManageRoles)
}

// below returns the role if the member may manage it: it is below their
// highest role (the owner outranks every role).
func (a access) below(roleID uuid.UUID) (store.ServerRole, error) {
	role, ok := a.role(roleID)
	if !ok {
		return store.ServerRole{}, ErrNoRole
	}
	if !permissions.Outranks(a.top(), int(role.Position), a.owner()) {
		return store.ServerRole{}, ErrForbidden
	}
	return role, nil
}

func validRole(spec RoleSpec) (string, error) {
	name, err := cleanName(spec.Name)
	if err != nil || (spec.Color != nil && (*spec.Color < 0 || *spec.Color > 0xFFFFFF)) {
		return "", ErrInvalid
	}
	return name, nil
}

// CreateRole adds a role just above @everyone, below every other role
// (MANAGE_ROLES; only permissions the member has).
func (service *Service) CreateRole(ctx context.Context, scope Scope, spec RoleSpec) (uuid.UUID, error) {
	a, err := service.managingRoles(ctx, scope)
	if err != nil {
		return uuid.Nil, err
	}
	name, err := validRole(spec)
	if err != nil {
		return uuid.Nil, err
	}
	if !a.mayGrant(spec.Permissions) {
		return uuid.Nil, ErrForbidden
	}
	id := uuid.Must(uuid.NewV7())
	order := []uuid.UUID{id}
	for _, role := range a.roles {
		if !role.Everyone {
			order = append(order, role.ID)
		}
	}
	if err := service.store.InsertRole(ctx, store.InsertRoleParams{
		ID: id, ServerID: scope.ServerID, Name: name, Color: spec.Color, Permissions: int64(spec.Permissions), Position: int32(len(order)), //nolint:gosec // role counts are small
	}); err != nil {
		return uuid.Nil, err
	}
	if err := service.store.OrderRoles(ctx, order); err != nil {
		return uuid.Nil, err
	}
	return id, service.notify(ctx, scope.ServerID)
}

// UpdateRole changes a role below the member's highest; @everyone counts
// as below everyone (MANAGE_ROLES; only permissions the member has).
func (service *Service) UpdateRole(ctx context.Context, scope Scope, roleID uuid.UUID, spec RoleSpec) error {
	a, err := service.managingRoles(ctx, scope)
	if err != nil {
		return err
	}
	role, err := a.below(roleID)
	if err != nil {
		return err
	}
	name, err := validRole(spec)
	if err != nil {
		return err
	}
	changed := permissions.Permission(role.Permissions) ^ spec.Permissions //nolint:gosec // stored non-negative
	if !a.mayGrant(changed) {
		return ErrForbidden
	}
	if role.Everyone {
		name = role.Name
	}
	if err := service.store.UpdateRole(ctx, store.UpdateRoleParams{ID: roleID, Name: name, Color: spec.Color, Permissions: int64(spec.Permissions)}); err != nil { //nolint:gosec // permissions use 27 bits
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// DeleteRole removes a role below the member's highest; @everyone stays (MANAGE_ROLES).
func (service *Service) DeleteRole(ctx context.Context, scope Scope, roleID uuid.UUID) error {
	a, err := service.managingRoles(ctx, scope)
	if err != nil {
		return err
	}
	role, err := a.below(roleID)
	if err != nil {
		return err
	}
	if role.Everyone {
		return ErrForbidden
	}
	if err := service.store.DeleteRole(ctx, roleID); err != nil {
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// OrderRoles sets the order of the roles, lowest first, @everyone left
// out. Roles at or above the member's highest must keep their place
// (MANAGE_ROLES).
func (service *Service) OrderRoles(ctx context.Context, scope Scope, order []uuid.UUID) error {
	a, err := service.managingRoles(ctx, scope)
	if err != nil {
		return err
	}
	current := []uuid.UUID{}
	for _, role := range a.roles {
		if !role.Everyone {
			current = append(current, role.ID)
		}
	}
	if !samePeople(current, order) {
		return ErrBadOrder
	}
	for index, id := range order {
		role, _ := a.role(id)
		moved := current[index] != id
		if moved && !permissions.Outranks(a.top(), int(role.Position), a.owner()) {
			return ErrForbidden
		}
	}
	if err := service.store.OrderRoles(ctx, order); err != nil {
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// samePeople reports whether two lists hold the same IDs, each once.
func samePeople(current []uuid.UUID, order []uuid.UUID) bool {
	if len(current) != len(order) {
		return false
	}
	seen := map[uuid.UUID]int{}
	for _, id := range current {
		seen[id]++
	}
	for _, id := range order {
		seen[id]--
		if seen[id] < 0 {
			return false
		}
	}
	return true
}

// RoleChange gives a member a role, or takes it away.
type RoleChange struct {
	UserID uuid.UUID
	RoleID uuid.UUID
	Give   bool
}

// SetMemberRole gives or takes a role below the member's highest (MANAGE_ROLES).
func (service *Service) SetMemberRole(ctx context.Context, scope Scope, change RoleChange) error {
	targetID, roleID, give := change.UserID, change.RoleID, change.Give
	a, err := service.managingRoles(ctx, scope)
	if err != nil {
		return err
	}
	role, err := a.below(roleID)
	if err != nil {
		return err
	}
	if role.Everyone {
		return ErrForbidden
	}
	if err := service.validTarget(ctx, a, TargetMember, targetID); err != nil {
		return err
	}
	params := store.AddMemberRoleParams{ServerID: scope.ServerID, UserID: targetID, RoleID: roleID}
	if give {
		err = service.store.AddMemberRole(ctx, params)
	} else {
		err = service.store.RemoveMemberRole(ctx, store.RemoveMemberRoleParams(params))
	}
	if err != nil {
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// Member is someone in a server, with their roles.
type Member struct {
	UserID      uuid.UUID
	Username    string
	DisplayName string
	AvatarID    *uuid.UUID
	JoinedAt    time.Time
	RoleIDs     []uuid.UUID
}

// Members lists the server's members to another member.
func (service *Service) Members(ctx context.Context, scope Scope) ([]Member, error) {
	if _, err := service.load(ctx, scope); err != nil {
		return nil, err
	}
	rows, err := service.store.ListServerMembers(ctx, scope.ServerID)
	if err != nil {
		return nil, err
	}
	pairs, err := service.store.ListMemberRoles(ctx, scope.ServerID)
	if err != nil {
		return nil, err
	}
	roles := map[uuid.UUID][]uuid.UUID{}
	for _, pair := range pairs {
		roles[pair.UserID] = append(roles[pair.UserID], pair.RoleID)
	}
	members := make([]Member, 0, len(rows))
	for _, row := range rows {
		members = append(members, Member{
			UserID: row.UserID, Username: row.Username, DisplayName: row.DisplayName, AvatarID: row.AvatarID,
			JoinedAt: row.JoinedAt, RoleIDs: append([]uuid.UUID{}, roles[row.UserID]...),
		})
	}
	return members, nil
}
