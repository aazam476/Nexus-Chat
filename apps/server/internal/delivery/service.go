package delivery

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/conversations"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/mlsheader"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// MaximumMessageSize is the largest live or archive copy accepted.
const MaximumMessageSize = 4 * 1024 * 1024

// Errors returned to the caller.
var (
	ErrNotFound       = errors.New("there is no such conversation, or you are not in it")
	ErrNotAllowed     = errors.New("you cannot send in this conversation")
	ErrInvalidMessage = errors.New("not an MLS message for this conversation's group, or too large")
	ErrWrongEpoch     = errors.New("the message is not for the group's current epoch; fetch the latest commits and retry")
	ErrEpochConflict  = store.ErrEpochConflict
	ErrDuplicate      = store.ErrDuplicateMessage
)

// GroupState is the group as the delivery service tracks it.
type GroupState struct {
	GroupID   []byte
	Epoch     int64
	GroupInfo []byte
}

// Message is a live copy as devices fetch it.
type Message struct {
	ID        int64
	Kind      string
	Epoch     int64
	SenderID  *uuid.UUID
	Data      []byte
	CreatedAt time.Time
}

// Membership is what the delivery service asks the conversations feature.
type Membership interface {
	IsMember(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) (bool, error)
	MaySend(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) (bool, error)
	MemberIDs(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error)
	Lookup(ctx context.Context, conversationID uuid.UUID) (conversations.Group, error)
}

type deliveryStore interface {
	EnsureGroup(ctx context.Context, params store.EnsureGroupParams) (store.MlsGroup, error)
	Commit(ctx context.Context, params store.CommitParams) (int64, error)
	Send(ctx context.Context, params store.SendParams) (int64, error)
	InsertMessage(ctx context.Context, params store.InsertMessageParams) (int64, error)
	MessagesAfter(ctx context.Context, params store.MessagesAfterParams) ([]store.MessagesAfterRow, error)
	ArchiveBefore(ctx context.Context, params store.ArchiveBeforeParams) ([]store.ArchiveBeforeRow, error)
	readStore
}

// Notifier tells member devices that a conversation has new MLS messages,
// and a user's devices that their read position moved.
type Notifier interface {
	MessagesArrived(ctx context.Context, userIDs []uuid.UUID, conversationID uuid.UUID, lastID int64)
	ReadNotifier
}

// Dependencies are what the service needs. DirectiveKey signs membership
// directives; it is the instance's federation key, which is public.
type Dependencies struct {
	Store        deliveryStore
	Members      Membership
	Notifier     Notifier
	DirectiveKey ed25519.PrivateKey
	Clock        clock.Clock
}

// Service is the delivery service.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

// group returns the conversation's state if the caller is a member.
func (service *Service) group(ctx context.Context, me uuid.UUID, conversationID uuid.UUID) (GroupState, error) {
	isMember, err := service.dependencies.Members.IsMember(ctx, conversationID, me)
	if err != nil {
		return GroupState{}, err
	}
	if !isMember {
		return GroupState{}, ErrNotFound
	}
	return service.state(ctx, conversationID)
}

func (service *Service) state(ctx context.Context, conversationID uuid.UUID) (GroupState, error) {
	conversation, err := service.dependencies.Members.Lookup(ctx, conversationID)
	if err != nil {
		return GroupState{}, err
	}
	row, err := service.dependencies.Store.EnsureGroup(ctx, store.EnsureGroupParams{ConversationID: conversationID, UpdatedAt: service.dependencies.Clock.Now()})
	if err != nil {
		return GroupState{}, err
	}
	return GroupState{GroupID: conversation.MLSGroupID, Epoch: row.Epoch, GroupInfo: row.GroupInfo}, nil
}

// State returns the group's epoch and latest GroupInfo.
func (service *Service) State(ctx context.Context, me uuid.UUID, conversationID uuid.UUID) (GroupState, error) {
	return service.group(ctx, me, conversationID)
}

// header parses message and checks it belongs to the group, has the
// expected content type, and fits the size limit.
func header(message []byte, group GroupState, contentType uint8) (mlsheader.Header, error) {
	if len(message) > MaximumMessageSize {
		return mlsheader.Header{}, ErrInvalidMessage
	}
	parsed, err := mlsheader.Parse(message)
	if err != nil || !bytes.Equal(parsed.GroupID, group.GroupID) || parsed.ContentType != contentType {
		return mlsheader.Header{}, ErrInvalidMessage
	}
	if parsed.Epoch != uint64(group.Epoch) { //nolint:gosec // epochs are never negative (database CHECK)
		return mlsheader.Header{}, ErrWrongEpoch
	}
	return parsed, nil
}

func (service *Service) notify(ctx context.Context, conversationID uuid.UUID, lastID int64, extra ...uuid.UUID) error {
	members, err := service.dependencies.Members.MemberIDs(ctx, conversationID)
	if err != nil {
		return err
	}
	service.dependencies.Notifier.MessagesArrived(ctx, append(members, extra...), conversationID, lastID)
	return nil
}
