package realtime

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Typing (docs/realtime-protocol.md §4–5): a device says its user is typing
// in a conversation, and the other members' devices get typing_start. The
// server keeps no typing state; devices show it for a few seconds.

const typeTyping = "typing"

// TypeTypingStart is the event that another member of a conversation is typing.
const TypeTypingStart = "typing_start"

const (
	// typingRepeat is how often one connection may announce typing in the
	// same conversation. Devices repeat it every 3 seconds; the slack
	// keeps a punctual repeat from being dropped.
	typingRepeat = 2500 * time.Millisecond
	// typingRemembered bounds the conversations a connection's throttle
	// keeps before forgetting old ones.
	typingRemembered = 64
)

// TypingRelay says who hears that a user is typing: the conversation's
// other members, or nobody when the user may not send there.
type TypingRelay interface {
	TypingRecipients(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) ([]uuid.UUID, error)
}

type typingData struct {
	ConversationID string `json:"conversationId"`
}

type typingStartData struct {
	ConversationID uuid.UUID `json:"conversationId"`
	UserID         uuid.UUID `json:"userId"`
}

// typingThrottle remembers when this connection last announced typing in
// each conversation. Only the connection's read loop uses it.
type typingThrottle struct {
	last map[uuid.UUID]time.Time
}

// allow reports whether typing in conversationID may be announced at now.
func (throttle *typingThrottle) allow(conversationID uuid.UUID, now time.Time) bool {
	if throttle.last == nil || len(throttle.last) >= typingRemembered {
		throttle.last = map[uuid.UUID]time.Time{}
	}
	if previous, seen := throttle.last[conversationID]; seen && now.Sub(previous) < typingRepeat {
		return false
	}
	throttle.last[conversationID] = now
	return true
}

// typing handles a typing frame.
func (connection *connection) typing(ctx context.Context, session *gatewaySession, frame clientFrame, throttle *typingThrottle) *closeReason {
	var data typingData
	if err := decodeData(frame, &data); err != nil {
		return &closeReason{closeMalformed, "malformed typing"}
	}
	conversationID, err := uuid.Parse(data.ConversationID)
	if err != nil {
		connection.send(ctx, serverFrame{Type: typeError, ID: frame.ID, Data: errorData{Code: "invalid_conversation", Message: "conversationId must be a UUID"}})
		return nil
	}
	gateway := connection.gateway
	if !throttle.allow(conversationID, gateway.options.Hub.clock.Now()) {
		return nil
	}
	recipients, err := gateway.options.Typing.TypingRecipients(ctx, conversationID, session.userID)
	if err != nil {
		connection.logger().ErrorContext(ctx, "finding who hears typing failed", slog.Any("error", err))
		return nil
	}
	gateway.options.Hub.SendToUsers(recipients, TypeTypingStart, typingStartData{ConversationID: conversationID, UserID: session.userID})
	return nil
}
