package social

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/social/store"
)

// How one user stands towards another, as that user sees it.
const (
	RelationshipNone     = "none"
	RelationshipFriend   = "friend"
	RelationshipIncoming = "incoming"
	RelationshipOutgoing = "outgoing"
	// RelationshipBlocked is shown only to the user who blocked; the
	// blocked user sees none, so a block is not announced.
	RelationshipBlocked = "blocked"
)

// Errors returned to the caller.
var (
	ErrUserNotFound     = errors.New("there is no such user")
	ErrSelf             = errors.New("that is you")
	ErrAlreadyFriends   = errors.New("you are already friends")
	ErrAlreadyRequested = errors.New("you already sent this user a friend request")
	// ErrNotAllowed covers both a block and the recipient's privacy
	// setting, so the sender can't tell which.
	ErrNotAllowed  = errors.New("this user does not accept friend requests from you")
	ErrNotFound    = errors.New("there is no such request, friendship or block")
	ErrRateLimited = errors.New("too many friend requests; wait a minute")
)

// Contact is another user in one of the caller's lists.
type Contact struct {
	UserID      uuid.UUID
	Username    string
	DisplayName string
	AvatarID    *uuid.UUID
	// Since is when the friendship, request or block began.
	Since time.Time
}

// Friends is the caller's friends and pending requests.
type Friends struct {
	Friends  []Contact
	Incoming []Contact
	Outgoing []Contact
}

type socialStore interface {
	FindUserByUsername(ctx context.Context, username string) (uuid.UUID, error)
	UserExists(ctx context.Context, id uuid.UUID) (bool, error)
	Relationship(ctx context.Context, params store.RelationshipParams) (store.RelationshipRow, error)
	InsertFriendRequest(ctx context.Context, params store.InsertFriendRequestParams) error
	AcceptFriendRequest(ctx context.Context, params store.AcceptFriendRequestParams) (int64, error)
	DeleteFriendRequest(ctx context.Context, params store.DeleteFriendRequestParams) (int64, error)
	DeleteFriendship(ctx context.Context, params store.DeleteFriendshipParams) (int64, error)
	Block(ctx context.Context, params store.BlockParams) error
	Unblock(ctx context.Context, params store.UnblockParams) (int64, error)
	ListFriends(ctx context.Context, userID uuid.UUID) ([]store.ListFriendsRow, error)
	ListIncomingRequests(ctx context.Context, recipientID uuid.UUID) ([]store.ListIncomingRequestsRow, error)
	ListOutgoingRequests(ctx context.Context, senderID uuid.UUID) ([]store.ListOutgoingRequestsRow, error)
	ListBlocked(ctx context.Context, blockerID uuid.UUID) ([]store.ListBlockedRow, error)
	GetPrivacy(ctx context.Context, userID uuid.UUID) (store.GetPrivacyRow, error)
	PutPrivacy(ctx context.Context, params store.PutPrivacyParams) error
}

// Notifier tells a user's devices that their relationship with another
// user changed. The realtime hub implements it.
type Notifier interface {
	RelationshipChanged(ctx context.Context, userID uuid.UUID, otherID uuid.UUID, relationship string)
}

// ServerMembership answers whether two users share a server, for the
// "server members" privacy choices. Servers arrive in M5; until then
// NoServers answers no.
type ServerMembership interface {
	ShareAServer(ctx context.Context, first uuid.UUID, second uuid.UUID) (bool, error)
}

// NoServers is the ServerMembership of an instance without servers.
type NoServers struct{}

// ShareAServer always answers false.
func (NoServers) ShareAServer(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

type attemptLimiter interface {
	Allow(key string) bool
}

// Dependencies are what the service needs. Limiter limits friend requests
// per sender, so nobody can flood others with them.
type Dependencies struct {
	Store    socialStore
	Notifier Notifier
	Servers  ServerMembership
	Limiter  attemptLimiter
	Clock    clock.Clock
}

// Service is the social graph.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

func (service *Service) relationship(ctx context.Context, me uuid.UUID, other uuid.UUID) (store.RelationshipRow, error) {
	return service.dependencies.Store.Relationship(ctx, store.RelationshipParams{Me: me, Other: other})
}

// change is how two users stand after an action, each from their side.
type change struct {
	me     uuid.UUID
	mine   string
	other  uuid.UUID
	theirs string
}

// notify tells both users how they now stand.
func (service *Service) notify(ctx context.Context, update change) {
	service.dependencies.Notifier.RelationshipChanged(ctx, update.me, update.other, update.mine)
	service.dependencies.Notifier.RelationshipChanged(ctx, update.other, update.me, update.theirs)
}

// existing returns ErrUserNotFound unless the user exists, and ErrSelf for the caller.
func (service *Service) existing(ctx context.Context, me uuid.UUID, other uuid.UUID) error {
	if me == other {
		return ErrSelf
	}
	exists, err := service.dependencies.Store.UserExists(ctx, other)
	if err != nil {
		return err
	}
	if !exists {
		return ErrUserNotFound
	}
	return nil
}
