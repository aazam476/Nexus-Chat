package delivery

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
)

// directiveLabel separates directive signatures from anything else the
// instance key signs.
const directiveLabel = "nexus-chat membership directive v1\n"

// Kinds of directive.
const (
	// DirectiveRemove tells members to commit a user's removal.
	DirectiveRemove = "remove"
	// DirectiveDelete tells members that a moderator deleted a message.
	DirectiveDelete = "delete"
	// DirectivePurge tells members that a user's messages since a time were deleted.
	DirectivePurge = "purge"
)

// Directive tells members' devices to commit a removal (docs/e2ee-design.md
// §3.2), or that a moderator deleted messages (spec §4.7). Devices check
// the signature with the instance's published key and reject removals of
// other users, and deletions, that no directive backs.
type Directive struct {
	Type     string    `json:"type"`
	GroupID  []byte    `json:"groupId"`
	UserID   uuid.UUID `json:"userId"`
	Reason   string    `json:"reason"`
	IssuedAt time.Time `json:"issuedAt"`
	// MessageID and ChannelID name a deleted message.
	MessageID *uuid.UUID `json:"messageId,omitempty"`
	ChannelID *uuid.UUID `json:"channelId,omitempty"`
	// Since is when a purge's messages start.
	Since *time.Time `json:"since,omitempty"`
}

// SignedDirective is what is stored and delivered: the directive's exact
// JSON bytes and the signature over the label and those bytes.
type SignedDirective struct {
	Directive []byte `json:"directive"`
	Signature []byte `json:"signature"`
}

// SignDirective signs directive with key.
func SignDirective(key ed25519.PrivateKey, directive Directive) (SignedDirective, error) {
	payload, err := json.Marshal(directive)
	if err != nil {
		return SignedDirective{}, fmt.Errorf("encoding the directive: %w", err)
	}
	return SignedDirective{Directive: payload, Signature: ed25519.Sign(key, append([]byte(directiveLabel), payload...))}, nil
}

// VerifyDirective checks a directive against the instance's public key.
func VerifyDirective(key ed25519.PublicKey, signed SignedDirective) bool {
	return ed25519.Verify(key, append([]byte(directiveLabel), signed.Directive...), signed.Signature)
}

// MemberLeft publishes a directive removing userID's devices from the
// conversation's group. It implements conversations.MembershipListener.
func (service *Service) MemberLeft(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) error {
	return service.publish(ctx, conversationID, Directive{Type: DirectiveRemove, UserID: userID, Reason: "left"})
}

// publish signs a directive for the conversation's group, in its current
// epoch, and delivers it to the members.
func (service *Service) publish(ctx context.Context, conversationID uuid.UUID, directive Directive) error {
	group, err := service.state(ctx, conversationID)
	if err != nil {
		return err
	}
	directive.GroupID, directive.IssuedAt = group.GroupID, service.dependencies.Clock.Now()
	signed, err := SignDirective(service.dependencies.DirectiveKey, directive)
	if err != nil {
		return err
	}
	data, err := json.Marshal(signed)
	if err != nil {
		return fmt.Errorf("encoding the signed directive: %w", err)
	}
	id, err := service.dependencies.Store.InsertMessage(ctx, store.InsertMessageParams{
		ConversationID: conversationID, Kind: "directive", Epoch: group.Epoch, Data: data, CreatedAt: service.dependencies.Clock.Now(),
	})
	if err != nil {
		return err
	}
	return service.notify(ctx, conversationID, id)
}
