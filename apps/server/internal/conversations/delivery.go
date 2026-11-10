package conversations

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Group is what the delivery service needs to know about a conversation.
type Group struct {
	Kind       string
	MLSGroupID []byte
}

// Lookup returns a conversation's kind and MLS group ID.
func (service *Service) Lookup(ctx context.Context, id uuid.UUID) (Group, error) {
	row, err := service.dependencies.Store.GetConversation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	if err != nil {
		return Group{}, err
	}
	return Group{Kind: row.Kind, MLSGroupID: row.MlsGroupID}, nil
}

// MaySend reports whether userID may send in the conversation: they are a
// member and, in a DM, neither of the two has blocked the other.
func (service *Service) MaySend(ctx context.Context, id uuid.UUID, userID uuid.UUID) (bool, error) {
	members, err := service.dependencies.Store.MemberIDs(ctx, id)
	if err != nil {
		return false, err
	}
	isMember := false
	var other uuid.UUID
	for _, member := range members {
		if member == userID {
			isMember = true
		} else {
			other = member
		}
	}
	if !isMember {
		return false, nil
	}
	group, err := service.Lookup(ctx, id)
	if err != nil {
		return false, err
	}
	if group.Kind != KindDirectMessage {
		return true, nil
	}
	blocked, err := service.dependencies.Policy.Blocked(ctx, userID, other)
	if err != nil {
		return false, err
	}
	return !blocked, nil
}
