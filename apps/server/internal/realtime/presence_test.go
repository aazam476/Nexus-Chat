package realtime

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

var testNow = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func userWith(status string, text *string, expires *time.Time) store.User {
	return store.User{ID: uuid.MustParse("01950000-0000-7000-8000-000000000001"), Status: status, CustomStatusText: text, CustomStatusExpiresAt: expires}
}

func TestComputePresence(t *testing.T) {
	text := "At lunch"
	later, earlier := testNow.Add(time.Hour), testNow.Add(-time.Minute)
	custom := &api.CustomStatus{Text: text, ExpiresAt: &later}
	tests := []struct {
		name      string
		user      store.User
		connected int
		allIdle   bool
		forSelf   bool
		want      Presence
	}{
		{"no device", userWith("online", &text, &later), 0, true, false, Presence{Status: "offline"}},
		{"no device, own view", userWith("online", &text, &later), 0, true, true, Presence{Status: "offline", CustomStatus: custom}},
		{"online", userWith("online", nil, nil), 2, false, false, Presence{Status: "online"}},
		{"every device idle", userWith("online", nil, nil), 2, true, false, Presence{Status: "idle"}},
		{"dnd stays dnd when idle", userWith("dnd", nil, nil), 1, true, false, Presence{Status: "dnd"}},
		{"chosen idle", userWith("idle", nil, nil), 1, false, false, Presence{Status: "idle"}},
		{"invisible to others", userWith("invisible", &text, &later), 1, false, false, Presence{Status: "offline"}},
		{"invisible to self", userWith("invisible", &text, &later), 1, false, true, Presence{Status: "invisible", CustomStatus: custom}},
		{"custom status", userWith("online", &text, &later), 1, false, false, Presence{Status: "online", CustomStatus: custom}},
		{"no expiry", userWith("online", &text, nil), 1, false, false, Presence{Status: "online", CustomStatus: &api.CustomStatus{Text: text}}},
		{"expired", userWith("online", &text, &earlier), 1, false, false, Presence{Status: "online"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.want.UserID = test.user.ID
			got := computePresence(presenceInput{user: test.user, connected: test.connected, allIdle: test.allIdle, now: testNow}, test.forSelf)

			assert.Equal(t, test.want, got)
		})
	}
}

func TestSamePresence(t *testing.T) {
	later, other := testNow.Add(time.Hour), testNow.Add(2*time.Hour)
	base := Presence{Status: "online", CustomStatus: &api.CustomStatus{Text: "a", ExpiresAt: &later}}
	assert.True(t, samePresence(base, Presence{Status: "online", CustomStatus: &api.CustomStatus{Text: "a", ExpiresAt: &later}}))
	assert.True(t, samePresence(Presence{Status: "idle"}, Presence{Status: "idle"}))
	assert.False(t, samePresence(base, Presence{Status: "idle", CustomStatus: base.CustomStatus}))
	assert.False(t, samePresence(base, Presence{Status: "online"}))
	assert.False(t, samePresence(base, Presence{Status: "online", CustomStatus: &api.CustomStatus{Text: "b", ExpiresAt: &later}}))
	assert.False(t, samePresence(base, Presence{Status: "online", CustomStatus: &api.CustomStatus{Text: "a", ExpiresAt: &other}}))
	assert.False(t, samePresence(base, Presence{Status: "online", CustomStatus: &api.CustomStatus{Text: "a"}}))
	assert.True(t, samePresence(Presence{CustomStatus: &api.CustomStatus{Text: "a"}}, Presence{CustomStatus: &api.CustomStatus{Text: "a"}}))
}
