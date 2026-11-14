package servers

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// audiences is the server's audiences as they are before a change.
type audiences struct {
	ids             []uuid.UUID
	members         map[uuid.UUID]map[uuid.UUID]bool
	channelAudience map[uuid.UUID]uuid.UUID
}

func currentAudiences(snapshot store.AudienceSnapshot) audiences {
	current := audiences{members: map[uuid.UUID]map[uuid.UUID]bool{}, channelAudience: map[uuid.UUID]uuid.UUID{}}
	for _, audience := range snapshot.Audiences {
		current.ids = append(current.ids, audience.ID)
		current.members[audience.ID] = map[uuid.UUID]bool{}
	}
	for _, row := range snapshot.AudienceMembers {
		current.members[row.ConversationID][row.UserID] = true
	}
	for _, channel := range snapshot.Channels {
		if channel.AudienceID != nil {
			current.channelAudience[channel.ID] = *channel.AudienceID
		}
	}
	return current
}

// pick chooses the audience a group of channels keeps: the untaken one
// most of its channels are in, or else an unused one with exactly the
// group's people, which needs no change at all.
func (current audiences) pick(group audienceGroup, taken map[uuid.UUID]bool) (uuid.UUID, bool) {
	if id, found := current.mostShared(group, taken); found {
		return id, true
	}
	return current.unusedMatch(group, taken)
}

// mostShared is the untaken audience holding most of the group's channels.
func (current audiences) mostShared(group audienceGroup, taken map[uuid.UUID]bool) (uuid.UUID, bool) {
	counts := map[uuid.UUID]int{}
	for _, channelID := range group.channels {
		if id, ok := current.channelAudience[channelID]; ok && !taken[id] {
			counts[id]++
		}
	}
	best, bestCount := uuid.Nil, 0
	for _, id := range current.ids {
		count := counts[id]
		if count > bestCount || (count == bestCount && count > 0 && strings.Compare(id.String(), best.String()) < 0) {
			best, bestCount = id, count
		}
	}
	return best, bestCount > 0
}

// unusedMatch is an audience no channel uses whose members are exactly
// the group's people.
func (current audiences) unusedMatch(group audienceGroup, taken map[uuid.UUID]bool) (uuid.UUID, bool) {
	used := map[uuid.UUID]bool{}
	for _, id := range current.channelAudience {
		used[id] = true
	}
	for _, id := range current.ids {
		if !taken[id] && !used[id] && current.hasExactly(id, group.users) {
			return id, true
		}
	}
	return uuid.Nil, false
}

func (current audiences) hasExactly(audienceID uuid.UUID, users []uuid.UUID) bool {
	members := current.members[audienceID]
	if len(members) != len(users) {
		return false
	}
	for _, user := range users {
		if !members[user] {
			return false
		}
	}
	return true
}

// diff adds to the plan what makes the audience's members exactly users.
func (current audiences) diff(plan *store.AudiencePlan, audienceID uuid.UUID, users []uuid.UUID, now time.Time) {
	members := current.members[audienceID]
	want := map[uuid.UUID]bool{}
	for _, user := range users {
		want[user] = true
		if !members[user] {
			plan.Add = append(plan.Add, store.AddAudienceMemberParams{ConversationID: audienceID, UserID: user, JoinedAt: now})
		}
	}
	for _, user := range sortedUsers(members) {
		if !want[user] {
			plan.Remove = append(plan.Remove, store.RemoveAudienceMemberParams{ConversationID: audienceID, UserID: user})
		}
	}
}

// retire removes people who left the server from audiences no channel
// uses any more. Everyone else keeps them, with their history keys.
func (current audiences) retire(plan *store.AudiencePlan, taken map[uuid.UUID]bool, inServer map[uuid.UUID]bool) {
	for _, id := range current.ids {
		if taken[id] {
			continue
		}
		for _, user := range sortedUsers(current.members[id]) {
			if !inServer[user] {
				plan.Remove = append(plan.Remove, store.RemoveAudienceMemberParams{ConversationID: id, UserID: user})
			}
		}
	}
}

func sortedUsers(set map[uuid.UUID]bool) []uuid.UUID {
	users := make([]uuid.UUID, 0, len(set))
	for user := range set {
		users = append(users, user)
	}
	slices.SortFunc(users, func(x, y uuid.UUID) int { return strings.Compare(x.String(), y.String()) })
	return users
}
