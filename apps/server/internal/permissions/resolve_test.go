package permissions

import (
	"fmt"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// state is what one layer says about one permission.
type state int

const (
	unset state = iota
	allow
	deny
)

func (s state) override(p Permission) Override {
	switch s {
	case allow:
		return Override{Allow: p}
	case deny:
		return Override{Deny: p}
	default:
		return Override{}
	}
}

// layers is one combination of everything that decides SEND_MESSAGES.
type layers struct {
	owner, adminRole, everyoneBase, roleBase, timedOut bool
	everyone, role1, role2, member                     state
}

// expected says, from the most specific layer down, whether SEND_MESSAGES
// is allowed. It is written as the rule in docs/permissions.md reads, not
// as the code computes it, so the two check each other.
func expected(l layers) bool {
	switch {
	case l.owner || l.adminRole:
		return true
	case l.timedOut:
		return false
	case l.member != unset:
		return l.member == allow
	default:
		return fromRolesDown(l)
	}
}

// fromRolesDown is the rest of expected: role overrides, where any allow
// wins, then @everyone's override, then the server permissions.
func fromRolesDown(l layers) bool {
	switch {
	case l.role1 == allow || l.role2 == allow:
		return true
	case l.role1 == deny || l.role2 == deny:
		return false
	case l.everyone != unset:
		return l.everyone == allow
	default:
		return l.everyoneBase || l.roleBase
	}
}

func resolve(l layers) Permission {
	everyone := ViewChannel
	if l.everyoneBase {
		everyone |= SendMessages
	}
	var role Permission
	if l.roleBase {
		role |= SendMessages
	}
	if l.adminRole {
		role |= Administrator
	}
	member := Member{Owner: l.owner, Everyone: everyone, Roles: []Permission{role}, TimedOut: l.timedOut}
	channel := Channel{
		Everyone: l.everyone.override(SendMessages),
		Roles:    []Override{l.role1.override(SendMessages), l.role2.override(SendMessages)},
		Member:   l.member.override(SendMessages),
	}
	return InChannel(member, channel)
}

func TestEveryCombinationOfLayers(t *testing.T) {
	bools := []bool{false, true}
	states := []state{unset, allow, deny}
	cases := 0
	for _, owner := range bools {
		for _, admin := range bools {
			for _, everyoneBase := range bools {
				for _, roleBase := range bools {
					for _, timedOut := range bools {
						for _, everyone := range states {
							for _, role1 := range states {
								for _, role2 := range states {
									for _, member := range states {
										l := layers{owner, admin, everyoneBase, roleBase, timedOut, everyone, role1, role2, member}
										assert.Equal(t, expected(l), resolve(l).Has(SendMessages), "%+v", l)
										cases++
									}
								}
							}
						}
					}
				}
			}
		}
	}
	require.Equal(t, 2*2*2*2*2*3*3*3*3, cases)
}

func TestWithoutViewChannelNothingElseCounts(t *testing.T) {
	member := Member{Everyone: DefaultEveryone, Roles: []Permission{ManageMessages}}
	for _, hide := range []Channel{
		{Everyone: Override{Deny: ViewChannel}},
		{Roles: []Override{{Deny: ViewChannel}}},
		{Member: Override{Deny: ViewChannel}},
	} {
		assert.Equal(t, Permission(0), InChannel(member, hide), "%+v", hide)
	}
	// A role can let some members back in to a hidden channel; the member's own deny wins.
	hidden := Channel{Everyone: Override{Deny: ViewChannel}, Roles: []Override{{Allow: ViewChannel}}}
	assert.True(t, InChannel(member, hidden).Has(ViewChannel|SendMessages|ManageMessages))
	hidden.Member = Override{Deny: ViewChannel}
	assert.Equal(t, Permission(0), InChannel(member, hidden))
	// Administrators and the owner see every channel.
	assert.Equal(t, All, InChannel(Member{Roles: []Permission{Administrator}}, Channel{Member: Override{Deny: All}}))
	assert.Equal(t, All, InChannel(Member{Owner: true}, Channel{Member: Override{Deny: All}}))
}

func TestServerPermissions(t *testing.T) {
	assert.Equal(t, DefaultEveryone|KickMembers, Server(Member{Everyone: DefaultEveryone, Roles: []Permission{KickMembers}}))
	assert.Equal(t, All, Server(Member{Owner: true}))
	assert.Equal(t, All, Server(Member{Roles: []Permission{Administrator}}))
	assert.Equal(t, ViewChannel|ReadHistoryBeforeJoin, Server(Member{Everyone: DefaultEveryone, TimedOut: true}))
	assert.Equal(t, All, Server(Member{Owner: true, TimedOut: true}))
	// Timed out in a channel too: reading only.
	assert.Equal(t, ViewChannel|ReadHistoryBeforeJoin, InChannel(Member{Everyone: All &^ Administrator, TimedOut: true}, Channel{}))
}

// The worked example in docs/permissions.md, step by step.
func TestTheWorkedExample(t *testing.T) {
	moderators := KickMembers | TimeoutMembers | ManageMessages
	// #announcements: @everyone may not post or react; Moderators may post.
	everyoneOverride := Override{Deny: SendMessages | AddReactions}
	moderatorsOverride := Override{Allow: SendMessages}
	dana := Member{Everyone: DefaultEveryone, Roles: []Permission{moderators}}
	sam := Member{Everyone: DefaultEveryone}

	assert.Equal(t, DefaultEveryone|moderators, Server(dana))
	// Dana has the Moderators role, so its override counts for her; Sam has no roles.
	danaThere := InChannel(dana, Channel{Everyone: everyoneOverride, Roles: []Override{moderatorsOverride}})
	assert.True(t, danaThere.Has(SendMessages|ManageMessages|ViewChannel))
	assert.False(t, danaThere.Has(AddReactions))
	samThere := InChannel(sam, Channel{Everyone: everyoneOverride})
	assert.False(t, samThere.Has(SendMessages))
	assert.True(t, samThere.Has(ViewChannel|ReadHistoryBeforeJoin))
	sam.TimedOut = true
	assert.Equal(t, ViewChannel|ReadHistoryBeforeJoin, InChannel(sam, Channel{}))
}

// Properties that must hold for any roles and overrides.
func TestResolutionProperties(t *testing.T) {
	limit := func(p uint64) Permission { return Permission(p) & All &^ Administrator }
	config := &quick.Config{MaxCount: 5000}

	// The order of a member's roles, and of their overrides, never matters.
	sameInAnyOrder := func(everyone, a, b uint64, oa, da, ob, db uint64) bool {
		member := Member{Everyone: limit(everyone), Roles: []Permission{limit(a), limit(b)}}
		swapped := Member{Everyone: member.Everyone, Roles: []Permission{limit(b), limit(a)}}
		one := Channel{Roles: []Override{{limit(oa), limit(da)}, {limit(ob), limit(db)}}}
		two := Channel{Roles: []Override{{limit(ob), limit(db)}, {limit(oa), limit(da)}}}
		return InChannel(member, one) == InChannel(swapped, two)
	}
	require.NoError(t, quick.Check(sameInAnyOrder, config))

	// Adding a role, or an allow, never takes a permission away.
	more := func(everyone, a, extra, allowed uint64) bool {
		before := InChannel(Member{Everyone: limit(everyone), Roles: []Permission{limit(a)}}, Channel{})
		after := InChannel(
			Member{Everyone: limit(everyone), Roles: []Permission{limit(a), limit(extra)}},
			Channel{Member: Override{Allow: limit(allowed)}},
		)
		return after.Has(before)
	}
	require.NoError(t, quick.Check(more, config))

	// A member's own deny always wins over every role, for everyone but administrators and the owner.
	ownDenyWins := func(everyone, a, allowed, denied uint64) bool {
		member := Member{Everyone: limit(everyone), Roles: []Permission{limit(a)}}
		got := InChannel(member, Channel{Roles: []Override{{Allow: limit(allowed)}}, Member: Override{Deny: limit(denied)}})
		return got&limit(denied) == 0
	}
	require.NoError(t, quick.Check(ownDenyWins, config))

	// Results never contain unknown bits, whatever is stored.
	known := func(everyone, a uint64, allowed uint64) bool {
		return InChannel(Member{Everyone: Permission(everyone), Roles: []Permission{Permission(a)}},
			Channel{Member: Override{Allow: Permission(allowed)}})&^All == 0
	}
	require.NoError(t, quick.Check(known, config))
}

func TestNames(t *testing.T) {
	require.Len(t, Names, 27, "spec §4.4 lists 27 permissions")
	assert.Equal(t, All, Permission(1<<27-1))
	assert.Equal(t, []string{"ADMINISTRATOR", "MOVE_MEMBERS"}, (Administrator | MoveMembers).Strings())
	parsed, ok := Parse([]string{"view_channel", "SEND_MESSAGES"})
	assert.True(t, ok)
	assert.Equal(t, ViewChannel|SendMessages, parsed)
	_, ok = Parse([]string{"FLY"})
	assert.False(t, ok)
	for bit, name := range Names {
		p, ok := Parse([]string{name})
		assert.True(t, ok, name)
		assert.Equal(t, Permission(1)<<bit, p, fmt.Sprint(name))
	}
}

func TestOutranks(t *testing.T) {
	assert.True(t, Outranks(3, 2, false))
	assert.False(t, Outranks(2, 2, false))
	assert.False(t, Outranks(1, 2, false))
	assert.True(t, Outranks(0, 5, true))
}
