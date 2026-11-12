package conversations

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/conversations/store"
)

// pairKey is the two user IDs in order, so a pair has one DM.
func pairKey(first uuid.UUID, second uuid.UUID) string {
	ids := []string{first.String(), second.String()}
	slices.Sort(ids)
	return ids[0] + ":" + ids[1]
}

// OpenDirectMessage returns the caller's DM with other, creating it if
// needed (true when created). An existing DM is returned even if one of
// them has since blocked the other; sending is what a block stops.
func (service *Service) OpenDirectMessage(ctx context.Context, me uuid.UUID, other uuid.UUID) (Conversation, bool, error) {
	if me == other {
		return Conversation{}, false, ErrSelf
	}
	pair := pairKey(me, other)
	existing, err := service.dependencies.Store.FindDirectMessage(ctx, &pair)
	if err == nil {
		conversation, err := service.Get(ctx, me, existing)
		return conversation, false, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, false, err
	}
	if err := service.allow(ctx, me, other); err != nil {
		return Conversation{}, false, err
	}
	conversation, err := service.create(ctx, me, store.InsertConversationParams{Kind: KindDirectMessage, DmPair: &pair}, []uuid.UUID{me, other})
	return conversation, err == nil, err
}

// CreateGroup starts a group DM of the caller and memberIDs.
func (service *Service) CreateGroup(ctx context.Context, me uuid.UUID, name *string, memberIDs []uuid.UUID) (Conversation, error) {
	if err := validName(name); err != nil {
		return Conversation{}, err
	}
	members := []uuid.UUID{me}
	for _, id := range memberIDs {
		if !slices.Contains(members, id) {
			members = append(members, id)
		}
	}
	if len(members) < MinimumGroupMembers || len(members) > service.dependencies.MaximumGroupMembers {
		return Conversation{}, ErrGroupSize
	}
	for _, id := range members[1:] {
		if err := service.allow(ctx, me, id); err != nil {
			return Conversation{}, err
		}
	}
	return service.create(ctx, me, store.InsertConversationParams{Kind: KindGroup, Name: name}, members)
}

func validName(name *string) error {
	if name != nil && (len([]rune(*name)) < 1 || len([]rune(*name)) > 100) {
		return ErrInvalidName
	}
	return nil
}

func (service *Service) create(ctx context.Context, me uuid.UUID, params store.InsertConversationParams, members []uuid.UUID) (Conversation, error) {
	now := service.dependencies.Clock.Now()
	params.ID = uuid.New()
	params.MlsGroupID = newGroupID()
	params.CreatedBy = &me
	params.CreatedAt = now
	if err := service.dependencies.Store.Create(ctx, params, members, now); err != nil {
		return Conversation{}, err
	}
	service.dependencies.Notifier.ConversationChanged(ctx, members, params.ID)
	return service.Get(ctx, me, params.ID)
}
