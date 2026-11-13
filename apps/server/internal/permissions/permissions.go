// Package permissions decides what a server member may do (spec §4.4).
// It is the only place permissions are resolved; the algorithm and a worked
// example are in docs/permissions.md. Everything here is pure: callers load
// the member's roles and the channel's overrides and pass them in.
package permissions

import "strings"

// Permission is a set of permissions, one bit each.
type Permission uint64

// The complete list for beta (spec §4.4). The bit positions are stored in
// the database and must never change; new permissions go at the end.
const (
	Administrator Permission = 1 << iota
	ManageServer
	ManageRoles
	ManageChannels
	ManageEmoji
	ViewAuditLog
	CreateInvites
	KickMembers
	BanMembers
	TimeoutMembers
	ViewChannel
	SendMessages
	SendInThreads
	CreateThreads
	ReadHistoryBeforeJoin
	AttachFiles
	AddReactions
	UseCustomEmoji
	MentionEveryone
	ManageMessages
	ConnectVoice
	Speak
	Video
	ScreenShare
	MuteMembers
	DeafenMembers
	MoveMembers

	// All is every permission there is.
	All = MoveMembers<<1 - 1
)

// Names are the permissions' names in the API and in docs/permissions.md, by bit.
var Names = []string{ //nolint:gochecknoglobals // a fixed table, read only
	"ADMINISTRATOR", "MANAGE_SERVER", "MANAGE_ROLES", "MANAGE_CHANNELS", "MANAGE_EMOJI",
	"VIEW_AUDIT_LOG", "CREATE_INVITES", "KICK_MEMBERS", "BAN_MEMBERS", "TIMEOUT_MEMBERS",
	"VIEW_CHANNEL", "SEND_MESSAGES", "SEND_IN_THREADS", "CREATE_THREADS",
	"READ_HISTORY_BEFORE_JOIN", "ATTACH_FILES", "ADD_REACTIONS", "USE_CUSTOM_EMOJI",
	"MENTION_EVERYONE", "MANAGE_MESSAGES", "CONNECT_VOICE", "SPEAK", "VIDEO",
	"SCREEN_SHARE", "MUTE_MEMBERS", "DEAFEN_MEMBERS", "MOVE_MEMBERS",
}

// DefaultEveryone is what the @everyone role of a new server allows:
// taking part, but nothing that manages the server or other members.
const DefaultEveryone = ViewChannel | SendMessages | SendInThreads | CreateThreads |
	ReadHistoryBeforeJoin | AttachFiles | AddReactions | UseCustomEmoji | CreateInvites |
	ConnectVoice | Speak | Video | ScreenShare

// whileTimedOut is what a timed-out member keeps: reading, nothing more.
const whileTimedOut = ViewChannel | ReadHistoryBeforeJoin

// Has reports whether p includes every permission in want.
func (p Permission) Has(want Permission) bool {
	return p&want == want
}

// Strings lists the names of the permissions in p, in bit order.
func (p Permission) Strings() []string {
	names := []string{}
	for bit, name := range Names {
		if p&(1<<bit) != 0 {
			names = append(names, name)
		}
	}
	return names
}

// Parse turns names into a Permission; ok is false if any name is unknown.
func Parse(names []string) (Permission, bool) {
	var p Permission
	for _, name := range names {
		bit := indexOf(strings.ToUpper(name))
		if bit < 0 {
			return 0, false
		}
		p |= 1 << bit
	}
	return p, true
}

func indexOf(name string) int {
	for bit, known := range Names {
		if known == name {
			return bit
		}
	}
	return -1
}
