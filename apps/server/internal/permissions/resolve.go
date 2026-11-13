package permissions

// Override is a channel's allow and deny for one role or one member. A
// permission in neither is left as it was.
type Override struct {
	Allow Permission
	Deny  Permission
}

// apply lets o change p: its denies come off, then its allows go on.
func (o Override) apply(p Permission) Permission {
	return p&^o.Deny | o.Allow
}

// Channel is what a channel says about one member: the @everyone
// override, the overrides of the member's roles, and the member's own.
type Channel struct {
	Everyone Override
	Roles    []Override
	Member   Override
}

// Member is everything resolution needs to know about one member.
type Member struct {
	// Owner is the server's owner, who may always do everything.
	Owner bool
	// Everyone is the @everyone role's permissions; Roles are the member's other roles'.
	Everyone Permission
	Roles    []Permission
	// TimedOut members may only read until the timeout ends.
	TimedOut bool
}

// Server is what the member may do in the server, outside any channel.
func Server(m Member) Permission {
	base := m.Everyone & All
	for _, role := range m.Roles {
		base |= role & All
	}
	if m.Owner || base.Has(Administrator) {
		return All
	}
	if m.TimedOut {
		return base & whileTimedOut
	}
	return base
}

// InChannel is what the member may do in a channel (docs/permissions.md):
// the server permissions, changed by the channel's @everyone override, then
// by all of the member's role overrides together (denies, then allows),
// then by the member's own override. A member who cannot view the channel
// can do nothing in it.
func InChannel(m Member, c Channel) Permission {
	base := Server(Member{Owner: m.Owner, Everyone: m.Everyone, Roles: m.Roles})
	if base == All {
		return All
	}
	base = c.Everyone.apply(base)
	var roles Override
	for _, role := range c.Roles {
		roles.Allow |= role.Allow
		roles.Deny |= role.Deny
	}
	base = c.Member.apply(roles.apply(base)) & All
	if !base.Has(ViewChannel) {
		return 0
	}
	if m.TimedOut {
		return base & whileTimedOut
	}
	return base
}

// Outranks reports whether a member whose highest role is at position
// actor may manage one whose highest is at target: roles are ordered, and
// a higher position is more senior. Only the owner manages equals.
func Outranks(actor, target int, actorIsOwner bool) bool {
	return actorIsOwner || actor > target
}
