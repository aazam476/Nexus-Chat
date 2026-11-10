package realtime

import (
	"context"

	"github.com/google/uuid"
)

// SendToUsers queues an event for every connected device of each user.
// Users with no device connected get nothing; they fetch the current state
// over HTTP when they next connect.
func (hub *Hub) SendToUsers(userIDs []uuid.UUID, eventType string, data any) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	for _, userID := range userIDs {
		if state, found := hub.users[userID]; found {
			hub.sendLocked(state, eventType, data)
		}
	}
}

type relationshipData struct {
	UserID       uuid.UUID `json:"userId"`
	Relationship string    `json:"relationship"`
}

// RelationshipChanged tells userID's devices how they now stand towards
// otherID. It implements social.Notifier.
func (hub *Hub) RelationshipChanged(_ context.Context, userID uuid.UUID, otherID uuid.UUID, relationship string) {
	hub.SendToUsers([]uuid.UUID{userID}, TypeRelationshipUpdate, relationshipData{UserID: otherID, Relationship: relationship})
}
