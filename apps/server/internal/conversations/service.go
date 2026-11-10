package conversations

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/conversations/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// Kinds of conversation.
const (
	KindDirectMessage = "dm"
	KindGroup         = "group"
)

// Group DM size (spec §4.3: 3 to 25 members, counting the creator).
const (
	MinimumGroupMembers = 3
	MaximumGroupMembers = 25
)

// Errors returned to the caller.
var (
	ErrNotFound      = errors.New("there is no such conversation, or you are not in it")
	ErrUserNotFound  = errors.New("there is no such user")
	ErrSelf          = errors.New("you cannot start a DM with yourself")
	ErrNotAllowed    = errors.New("this user does not accept DMs from you")
	ErrGroupSize     = errors.New("a group DM has 3 to 25 members")
	ErrNotAGroup     = errors.New("only group DMs can be renamed, added to or left")
	ErrInvalidName   = errors.New("a group DM name is 1 to 100 characters")
	ErrAlreadyMember = errors.New("this user is already in the conversation")
)

// Member is a user in a conversation.
type Member struct {
	UserID      uuid.UUID
	Username    string
	DisplayName string
	AvatarID    *uuid.UUID
}

// Conversation is a DM or group DM as its members see it.
type Conversation struct {
	ID         uuid.UUID
	Kind       string
	Name       *string
	MLSGroupID []byte
	CreatedAt  time.Time
	Members    []Member
}

type conversationStore interface {
	Create(ctx context.Context, conversation store.InsertConversationParams, memberIDs []uuid.UUID, now time.Time) error
	FindDirectMessage(ctx context.Context, pair *string) (uuid.UUID, error)
	AddMember(ctx context.Context, params store.AddMemberParams) error
	RemoveMember(ctx context.Context, params store.RemoveMemberParams) (int64, error)
	GetConversation(ctx context.Context, id uuid.UUID) (store.Conversation, error)
	IsMember(ctx context.Context, params store.IsMemberParams) (bool, error)
	CountMembers(ctx context.Context, conversationID uuid.UUID) (int64, error)
	MemberIDs(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error)
	ListMembers(ctx context.Context, conversationIDs []uuid.UUID) ([]store.ListMembersRow, error)
	ListConversations(ctx context.Context, userID uuid.UUID) ([]store.Conversation, error)
	RenameConversation(ctx context.Context, params store.RenameConversationParams) error
	UserExists(ctx context.Context, id uuid.UUID) (bool, error)
}

// DirectMessagePolicy is the social feature's answer to who may DM whom.
type DirectMessagePolicy interface {
	CanDirectMessage(ctx context.Context, sender uuid.UUID, recipient uuid.UUID) (bool, error)
	Blocked(ctx context.Context, first uuid.UUID, second uuid.UUID) (bool, error)
}

// Notifier tells members' devices that a conversation changed.
type Notifier interface {
	ConversationChanged(ctx context.Context, userIDs []uuid.UUID, conversationID uuid.UUID)
}

// MembershipListener hears about members leaving, so their devices can be
// removed from the MLS group (the delivery service issues the directive).
type MembershipListener interface {
	MemberLeft(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) error
}

// Dependencies are what the service needs.
type Dependencies struct {
	Store    conversationStore
	Policy   DirectMessagePolicy
	Notifier Notifier
	Members  MembershipListener
	Clock    clock.Clock
}

// Service manages conversations.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

func newGroupID() []byte {
	id := make([]byte, 16)
	// crypto/rand.Read never returns an error; it stops the program if the
	// system's random source fails.
	_, _ = rand.Read(id)
	return id
}

// allow checks that recipient exists and accepts DMs from sender.
func (service *Service) allow(ctx context.Context, sender uuid.UUID, recipient uuid.UUID) error {
	exists, err := service.dependencies.Store.UserExists(ctx, recipient)
	if err != nil {
		return err
	}
	if !exists {
		return ErrUserNotFound
	}
	allowed, err := service.dependencies.Policy.CanDirectMessage(ctx, sender, recipient)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrNotAllowed
	}
	return nil
}

func (service *Service) notifyMembers(ctx context.Context, conversationID uuid.UUID, extra ...uuid.UUID) error {
	members, err := service.dependencies.Store.MemberIDs(ctx, conversationID)
	if err != nil {
		return err
	}
	service.dependencies.Notifier.ConversationChanged(ctx, append(members, extra...), conversationID)
	return nil
}
