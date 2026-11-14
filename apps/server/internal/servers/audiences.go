package servers

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// Audiences (docs/e2ee-design.md §3.1): every text channel is encrypted in
// the MLS group of its audience, the members who hold VIEW_CHANNEL on it.
// Channels with the same audience share one group. After every change to a
// server, its audiences are worked out again: each set of channels that
// share an audience keeps the group most of them are already in, so a new
// member or a role change only adds or removes people. A group no channel
// uses any more keeps its members, who still hold its history keys; only
// people who leave the server are removed from it.

// AudienceListener hears that a user must leave an audience's group. The
// delivery service implements it by publishing a removal directive.
type AudienceListener interface {
	MemberLeft(ctx context.Context, audienceID uuid.UUID, userID uuid.UUID) error
}

// audienceIdentity is a new audience's conversation ID and MLS group ID.
type audienceIdentity func() (uuid.UUID, []byte)

// reconcile brings the server's audiences in line with its permissions,
// then asks for the removals to be committed.
func (service *Service) reconcile(ctx context.Context, serverID uuid.UUID) error {
	now := service.clock.Now()
	plan, err := service.store.ReconcileAudiences(ctx, serverID, func(snapshot store.AudienceSnapshot) store.AudiencePlan {
		return planAudiences(snapshot, now, newAudienceIdentity)
	})
	if err != nil {
		return err
	}
	for _, removal := range plan.Remove {
		if err := service.audiences.MemberLeft(ctx, removal.ConversationID, removal.UserID); err != nil {
			return err
		}
	}
	return nil
}

func newAudienceIdentity() (uuid.UUID, []byte) {
	return uuid.Must(uuid.NewV7()), newMLSGroupID()
}

// audienceGroup is a set of text channels that share one audience.
type audienceGroup struct {
	key      string
	channels []uuid.UUID
	users    []uuid.UUID
}

// planAudiences works out how the snapshot's audiences must change.
func planAudiences(snapshot store.AudienceSnapshot, now time.Time, identity audienceIdentity) store.AudiencePlan {
	current := currentAudiences(snapshot)
	var plan store.AudiencePlan
	taken := map[uuid.UUID]bool{}
	for _, group := range groupByAudience(snapshot, now) {
		audienceID, found := current.pick(group, taken)
		if !found {
			id, mlsGroupID := identity()
			serverID := snapshot.Server.ID
			plan.Create = append(plan.Create, store.InsertAudienceParams{ID: id, MlsGroupID: mlsGroupID, ServerID: &serverID, CreatedAt: now})
			audienceID = id
		}
		taken[audienceID] = true
		for _, channelID := range group.channels {
			if current.channelAudience[channelID] != audienceID {
				plan.Assign = append(plan.Assign, store.SetChannelAudienceParams{ID: channelID, AudienceID: &audienceID})
			}
		}
		current.diff(&plan, audienceID, group.users, now)
	}
	current.retire(&plan, taken, memberSet(snapshot))
	return plan
}

// groupByAudience groups the text channels by who can see them: the groups
// with most channels first, then those with most people, so when a channel
// is hidden from some, the larger audience keeps the group and fewer
// people have to join a new one.
func groupByAudience(snapshot store.AudienceSnapshot, now time.Time) []audienceGroup {
	accesses := memberAccesses(snapshot, now)
	byKey := map[string]*audienceGroup{}
	for _, channel := range snapshot.Channels {
		if channel.Kind != KindText {
			continue
		}
		var users []uuid.UUID
		for _, a := range accesses {
			if a.inChannel(channel.ID).Has(permissions.ViewChannel) {
				users = append(users, a.userID)
			}
		}
		slices.SortFunc(users, func(x, y uuid.UUID) int { return strings.Compare(x.String(), y.String()) })
		key := audienceKey(users)
		if byKey[key] == nil {
			byKey[key] = &audienceGroup{key: key, users: users}
		}
		byKey[key].channels = append(byKey[key].channels, channel.ID)
	}
	groups := make([]audienceGroup, 0, len(byKey))
	for _, group := range byKey {
		groups = append(groups, *group)
	}
	slices.SortFunc(groups, func(x, y audienceGroup) int {
		if len(x.channels) != len(y.channels) {
			return len(y.channels) - len(x.channels)
		}
		if len(x.users) != len(y.users) {
			return len(y.users) - len(x.users)
		}
		return strings.Compare(x.key, y.key)
	})
	return groups
}

func audienceKey(users []uuid.UUID) string {
	parts := make([]string, 0, len(users))
	for _, user := range users {
		parts = append(parts, user.String())
	}
	return strings.Join(parts, ",")
}

// memberAccesses is every member's access, for permission checks.
func memberAccesses(snapshot store.AudienceSnapshot, now time.Time) []access {
	rolesOf := map[uuid.UUID]map[uuid.UUID]bool{}
	for _, row := range snapshot.MemberRoles {
		if rolesOf[row.UserID] == nil {
			rolesOf[row.UserID] = map[uuid.UUID]bool{}
		}
		rolesOf[row.UserID][row.RoleID] = true
	}
	accesses := make([]access, 0, len(snapshot.Members))
	for _, member := range snapshot.Members {
		accesses = append(accesses, access{
			server: snapshot.Server, userID: member.UserID, roles: snapshot.Roles, overrides: snapshot.Overrides,
			timedOut: member.TimedOutUntil != nil && member.TimedOutUntil.After(now), myRoles: rolesOf[member.UserID],
		})
	}
	return accesses
}

func memberSet(snapshot store.AudienceSnapshot) map[uuid.UUID]bool {
	members := map[uuid.UUID]bool{}
	for _, member := range snapshot.Members {
		members[member.UserID] = true
	}
	return members
}
