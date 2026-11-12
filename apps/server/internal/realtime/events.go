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

type conversationData struct {
	ConversationID uuid.UUID `json:"conversationId"`
}

type messagesData struct {
	ConversationID uuid.UUID `json:"conversationId"`
	LastID         int64     `json:"lastId"`
}

// ConversationChanged tells members that a conversation's name or
// members changed; they fetch it again. It implements conversations.Notifier.
func (hub *Hub) ConversationChanged(_ context.Context, userIDs []uuid.UUID, conversationID uuid.UUID) {
	hub.SendToUsers(userIDs, TypeConversationUpdate, conversationData{ConversationID: conversationID})
}

// MessagesArrived tells members that a conversation has MLS messages up to
// lastID; they fetch what they miss. It implements delivery.Notifier.
func (hub *Hub) MessagesArrived(_ context.Context, userIDs []uuid.UUID, conversationID uuid.UUID, lastID int64) {
	hub.SendToUsers(userIDs, TypeMlsMessages, messagesData{ConversationID: conversationID, LastID: lastID})
}

type readData struct {
	ConversationID uuid.UUID `json:"conversationId"`
	LastReadID     int64     `json:"lastReadId"`
}

// ReadChanged tells the user's devices how far they have now read the
// conversation, so every device clears the same unread markers.
func (hub *Hub) ReadChanged(_ context.Context, userID uuid.UUID, conversationID uuid.UUID, lastReadID int64) {
	hub.SendToUsers([]uuid.UUID{userID}, TypeReadUpdate, readData{ConversationID: conversationID, LastReadID: lastReadID})
}
