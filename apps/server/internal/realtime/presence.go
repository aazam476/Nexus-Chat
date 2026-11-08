package realtime

import (
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// Presence is what others see of a user (docs/realtime-protocol.md §5).
type Presence struct {
	UserID       uuid.UUID         `json:"userId"`
	Status       string            `json:"status"`
	CustomStatus *api.CustomStatus `json:"customStatus,omitempty"`
}

// presenceInput is everything presence depends on.
type presenceInput struct {
	user      store.User
	connected int
	allIdle   bool
	now       time.Time
}

// computePresence applies docs/realtime-protocol.md §6. forSelf is true
// for the user's own devices, which see "invisible" instead of "offline".
func computePresence(input presenceInput, forSelf bool) Presence {
	presence := Presence{UserID: input.user.ID, Status: input.user.Status}
	switch {
	case input.connected == 0:
		presence.Status = accounts.StatusOffline
	case input.user.Status == accounts.StatusOnline && input.allIdle:
		presence.Status = accounts.StatusIdle
	case input.user.Status == accounts.StatusInvisible && !forSelf:
		presence.Status = accounts.StatusOffline
	}
	if presence.Status != accounts.StatusOffline || forSelf {
		presence.CustomStatus = customStatus(input.user, input.now)
	}
	return presence
}

// customStatus is the user's custom status unless it has expired; the
// sweeper removes expired ones from the database within a minute.
func customStatus(user store.User, now time.Time) *api.CustomStatus {
	if user.CustomStatusText == nil || (user.CustomStatusExpiresAt != nil && !user.CustomStatusExpiresAt.After(now)) {
		return nil
	}
	return &api.CustomStatus{Text: *user.CustomStatusText, ExpiresAt: user.CustomStatusExpiresAt}
}

// samePresence compares two presences field by field.
func samePresence(first Presence, second Presence) bool {
	if first.Status != second.Status || (first.CustomStatus == nil) != (second.CustomStatus == nil) {
		return false
	}
	if first.CustomStatus == nil {
		return true
	}
	firstExpiry, secondExpiry := first.CustomStatus.ExpiresAt, second.CustomStatus.ExpiresAt
	sameExpiry := (firstExpiry == nil && secondExpiry == nil) ||
		(firstExpiry != nil && secondExpiry != nil && firstExpiry.Equal(*secondExpiry))
	return first.CustomStatus.Text == second.CustomStatus.Text && sameExpiry
}
