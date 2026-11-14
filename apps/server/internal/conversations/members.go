package conversations

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/conversations/store"
)

// Get returns a conversation the caller is in.
func (service *Service) Get(ctx context.Context, me uuid.UUID, id uuid.UUID) (Conversation, error) {
	row, err := service.member(ctx, me, id)
	if err != nil {
		return Conversation{}, err
	}
	withMembers, err := service.withMembers(ctx, []store.Conversation{row})
	if err != nil {
		return Conversation{}, err
	}
	return withMembers[0], nil
}

// List returns the caller's conversations, newest first.
func (service *Service) List(ctx context.Context, me uuid.UUID) ([]Conversation, error) {
	rows, err := service.dependencies.Store.ListConversations(ctx, me)
	if err != nil {
		return nil, err
	}
	return service.withMembers(ctx, rows)
}

// IsMember reports whether userID is in the conversation; the delivery
// service asks this before accepting or handing out MLS messages.
func (service *Service) IsMember(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) (bool, error) {
	return service.dependencies.Store.IsMember(ctx, store.IsMemberParams{ConversationID: conversationID, UserID: userID})
}

// MemberIDs returns the users in the conversation.
func (service *Service) MemberIDs(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	return service.dependencies.Store.MemberIDs(ctx, conversationID)
}

// member returns the conversation if the caller is in it, else ErrNotFound.
// Server audiences are not conversations to their members, so they are
// never found here.
func (service *Service) member(ctx context.Context, me uuid.UUID, id uuid.UUID) (store.Conversation, error) {
	isMember, err := service.IsMember(ctx, id, me)
	if err != nil {
		return store.Conversation{}, err
	}
	if !isMember {
		return store.Conversation{}, ErrNotFound
	}
	row, err := service.dependencies.Store.GetConversation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.Kind == KindAudience) {
		return store.Conversation{}, ErrNotFound
	}
	return row, err
}

// group returns a group DM the caller is in.
func (service *Service) group(ctx context.Context, me uuid.UUID, id uuid.UUID) (store.Conversation, error) {
	row, err := service.member(ctx, me, id)
	if err != nil {
		return store.Conversation{}, err
	}
	if row.Kind != KindGroup {
		return store.Conversation{}, ErrNotAGroup
	}
	return row, nil
}

func (service *Service) withMembers(ctx context.Context, rows []store.Conversation) ([]Conversation, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	memberRows, err := service.dependencies.Store.ListMembers(ctx, ids)
	if err != nil {
		return nil, err
	}
	byConversation := map[uuid.UUID][]Member{}
	for _, row := range memberRows {
		byConversation[row.ConversationID] = append(byConversation[row.ConversationID],
			Member{UserID: row.ID, Username: row.Username, DisplayName: row.DisplayName, AvatarID: row.AvatarID})
	}
	conversations := make([]Conversation, 0, len(rows))
	for _, row := range rows {
		conversations = append(conversations, Conversation{
			ID: row.ID, Kind: row.Kind, Name: row.Name, MLSGroupID: row.MlsGroupID, CreatedAt: row.CreatedAt, Members: byConversation[row.ID],
			IconID: row.IconID,
		})
	}
	return conversations, nil
}

// AddMember adds userID to a group DM the caller is in.
func (service *Service) AddMember(ctx context.Context, me uuid.UUID, id uuid.UUID, userID uuid.UUID) error {
	if _, err := service.group(ctx, me, id); err != nil {
		return err
	}
	already, err := service.IsMember(ctx, id, userID)
	if err != nil {
		return err
	}
	if already {
		return ErrAlreadyMember
	}
	count, err := service.dependencies.Store.CountMembers(ctx, id)
	if err != nil {
		return err
	}
	if count >= int64(service.dependencies.MaximumGroupMembers) {
		return ErrGroupSize
	}
	if err := service.allow(ctx, me, userID); err != nil {
		return err
	}
	if err := service.dependencies.Store.AddMember(ctx, store.AddMemberParams{ConversationID: id, UserID: userID, JoinedAt: service.dependencies.Clock.Now()}); err != nil {
		return err
	}
	return service.notifyMembers(ctx, id)
}

// Leave takes the caller out of a group DM. Their devices leave the MLS
// group through a removal directive the delivery service issues.
func (service *Service) Leave(ctx context.Context, me uuid.UUID, id uuid.UUID) error {
	if _, err := service.group(ctx, me, id); err != nil {
		return err
	}
	if _, err := service.dependencies.Store.RemoveMember(ctx, store.RemoveMemberParams{ConversationID: id, UserID: me}); err != nil {
		return err
	}
	if err := service.dependencies.Members.MemberLeft(ctx, id, me); err != nil {
		return err
	}
	return service.notifyMembers(ctx, id, me)
}

// Rename changes a group DM's name; nil removes it.
func (service *Service) Rename(ctx context.Context, me uuid.UUID, id uuid.UUID, name *string) error {
	if err := validName(name); err != nil {
		return err
	}
	if _, err := service.group(ctx, me, id); err != nil {
		return err
	}
	if err := service.dependencies.Store.RenameConversation(ctx, store.RenameConversationParams{ID: id, Name: name}); err != nil {
		return err
	}
	return service.notifyMembers(ctx, id)
}
