package accounts

// Statuses a user can choose (spec §4.1). Idle is also set automatically
// while every device of the user is inactive; offline is never stored and
// means no device is connected.
const (
	StatusOnline    = "online"
	StatusIdle      = "idle"
	StatusDND       = "dnd"
	StatusInvisible = "invisible"
	StatusOffline   = "offline"
)
