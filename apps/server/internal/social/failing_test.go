package social

import (
	"context"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/social/store"
)

// failing wraps the real store and fails the call named in failOn.
type failing struct {
	*store.Queries
	failOn string
}

func (fake failing) Relationship(ctx context.Context, params store.RelationshipParams) (store.RelationshipRow, error) {
	if fake.failOn == "Relationship" {
		return store.RelationshipRow{}, errDown
	}
	return fake.Queries.Relationship(ctx, params)
}

func (fake failing) GetPrivacy(ctx context.Context, userID uuid.UUID) (store.GetPrivacyRow, error) {
	if fake.failOn == "GetPrivacy" {
		return store.GetPrivacyRow{}, errDown
	}
	return fake.Queries.GetPrivacy(ctx, userID)
}

func (fake failing) failure(call string) error {
	if fake.failOn == call {
		return errDown
	}
	return nil
}

func (fake failing) FindUserByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	if err := fake.failure("FindUserByUsername"); err != nil {
		return uuid.Nil, err
	}
	return fake.Queries.FindUserByUsername(ctx, username)
}

func (fake failing) UserExists(ctx context.Context, id uuid.UUID) (bool, error) {
	if err := fake.failure("UserExists"); err != nil {
		return false, err
	}
	return fake.Queries.UserExists(ctx, id)
}

func (fake failing) InsertFriendRequest(ctx context.Context, params store.InsertFriendRequestParams) error {
	if err := fake.failure("InsertFriendRequest"); err != nil {
		return err
	}
	return fake.Queries.InsertFriendRequest(ctx, params)
}

func (fake failing) AcceptFriendRequest(ctx context.Context, params store.AcceptFriendRequestParams) (int64, error) {
	if err := fake.failure("AcceptFriendRequest"); err != nil {
		return 0, err
	}
	return fake.Queries.AcceptFriendRequest(ctx, params)
}

func (fake failing) DeleteFriendRequest(ctx context.Context, params store.DeleteFriendRequestParams) (int64, error) {
	if err := fake.failure("DeleteFriendRequest"); err != nil {
		return 0, err
	}
	return fake.Queries.DeleteFriendRequest(ctx, params)
}

func (fake failing) DeleteFriendship(ctx context.Context, params store.DeleteFriendshipParams) (int64, error) {
	if err := fake.failure("DeleteFriendship"); err != nil {
		return 0, err
	}
	return fake.Queries.DeleteFriendship(ctx, params)
}

func (fake failing) Block(ctx context.Context, params store.BlockParams) error {
	if err := fake.failure("Block"); err != nil {
		return err
	}
	return fake.Queries.Block(ctx, params)
}

func (fake failing) Unblock(ctx context.Context, params store.UnblockParams) (int64, error) {
	if err := fake.failure("Unblock"); err != nil {
		return 0, err
	}
	return fake.Queries.Unblock(ctx, params)
}

func (fake failing) ListFriends(ctx context.Context, userID uuid.UUID) ([]store.ListFriendsRow, error) {
	if err := fake.failure("ListFriends"); err != nil {
		return nil, err
	}
	return fake.Queries.ListFriends(ctx, userID)
}

func (fake failing) ListIncomingRequests(ctx context.Context, userID uuid.UUID) ([]store.ListIncomingRequestsRow, error) {
	if err := fake.failure("ListIncomingRequests"); err != nil {
		return nil, err
	}
	return fake.Queries.ListIncomingRequests(ctx, userID)
}

func (fake failing) ListOutgoingRequests(ctx context.Context, userID uuid.UUID) ([]store.ListOutgoingRequestsRow, error) {
	if err := fake.failure("ListOutgoingRequests"); err != nil {
		return nil, err
	}
	return fake.Queries.ListOutgoingRequests(ctx, userID)
}

func (fake failing) ListBlocked(ctx context.Context, userID uuid.UUID) ([]store.ListBlockedRow, error) {
	if err := fake.failure("ListBlocked"); err != nil {
		return nil, err
	}
	return fake.Queries.ListBlocked(ctx, userID)
}

func (fake failing) PutPrivacy(ctx context.Context, params store.PutPrivacyParams) error {
	if err := fake.failure("PutPrivacy"); err != nil {
		return err
	}
	return fake.Queries.PutPrivacy(ctx, params)
}

type failingServers struct{}

func (failingServers) ShareAServer(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, errDown
}
