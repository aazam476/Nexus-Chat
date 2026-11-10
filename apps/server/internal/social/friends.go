package social

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/social/store"
)

// SendRequest sends a friend request to the user called username. If that
// user had already sent one to the caller, they become friends instead.
// It returns the caller's new relationship with them and their ID.
func (service *Service) SendRequest(ctx context.Context, me uuid.UUID, username string) (string, uuid.UUID, error) {
	other, err := service.recipient(ctx, me, username)
	if err != nil {
		return "", uuid.Nil, err
	}
	theyAsked, err := service.mayRequest(ctx, me, other)
	if err != nil {
		return "", uuid.Nil, err
	}
	if theyAsked {
		return RelationshipFriend, other, service.Accept(ctx, me, other)
	}
	if err := service.dependencies.Store.InsertFriendRequest(ctx, store.InsertFriendRequestParams{
		SenderID: me, RecipientID: other, CreatedAt: service.dependencies.Clock.Now(),
	}); err != nil {
		return "", uuid.Nil, err
	}
	service.notify(ctx, change{me: me, mine: RelationshipOutgoing, other: other, theirs: RelationshipIncoming})
	return RelationshipOutgoing, other, nil
}

// recipient finds the user a request is for, within the sender's rate limit.
func (service *Service) recipient(ctx context.Context, me uuid.UUID, username string) (uuid.UUID, error) {
	if !service.dependencies.Limiter.Allow(me.String()) {
		return uuid.Nil, ErrRateLimited
	}
	other, err := service.dependencies.Store.FindUserByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrUserNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	if other == me {
		return uuid.Nil, ErrSelf
	}
	return other, nil
}

// mayRequest checks the caller may send other a request; true means other
// already sent one to the caller, so accepting it is the answer.
func (service *Service) mayRequest(ctx context.Context, me uuid.UUID, other uuid.UUID) (bool, error) {
	current, err := service.relationship(ctx, me, other)
	if err != nil {
		return false, err
	}
	switch {
	case current.Friends:
		return false, ErrAlreadyFriends
	case current.Outgoing:
		return false, ErrAlreadyRequested
	case current.IBlocked || current.BlockedMe:
		return false, ErrNotAllowed
	case current.Incoming:
		return true, nil
	}
	return false, service.allowFriendRequest(ctx, me, other)
}

// allowFriendRequest applies the recipient's "who can send me friend requests" setting.
func (service *Service) allowFriendRequest(ctx context.Context, sender uuid.UUID, recipient uuid.UUID) error {
	privacy, err := service.dependencies.Store.GetPrivacy(ctx, recipient)
	if err != nil {
		return err
	}
	switch privacy.WhoCanFriendRequest {
	case FriendRequestsFromEveryone:
		return nil
	case FriendRequestsFromServerMembers:
		shared, err := service.dependencies.Servers.ShareAServer(ctx, sender, recipient)
		if err != nil {
			return err
		}
		if shared {
			return nil
		}
	}
	return ErrNotAllowed
}

// Accept accepts the friend request other sent to the caller.
func (service *Service) Accept(ctx context.Context, me uuid.UUID, other uuid.UUID) error {
	accepted, err := service.dependencies.Store.AcceptFriendRequest(ctx, store.AcceptFriendRequestParams{
		Sender: other, Recipient: me, Now: service.dependencies.Clock.Now(),
	})
	if err != nil {
		return err
	}
	if accepted == 0 {
		return ErrNotFound
	}
	service.notify(ctx, change{me: me, mine: RelationshipFriend, other: other, theirs: RelationshipFriend})
	return nil
}

// DeleteRequest declines a request other sent to the caller, or cancels
// one the caller sent to other.
func (service *Service) DeleteRequest(ctx context.Context, me uuid.UUID, other uuid.UUID) error {
	declined, err := service.dependencies.Store.DeleteFriendRequest(ctx, store.DeleteFriendRequestParams{SenderID: other, RecipientID: me})
	if err != nil {
		return err
	}
	cancelled, err := service.dependencies.Store.DeleteFriendRequest(ctx, store.DeleteFriendRequestParams{SenderID: me, RecipientID: other})
	if err != nil {
		return err
	}
	if declined+cancelled == 0 {
		return ErrNotFound
	}
	service.notify(ctx, change{me: me, mine: RelationshipNone, other: other, theirs: RelationshipNone})
	return nil
}

// RemoveFriend ends the caller's friendship with other.
func (service *Service) RemoveFriend(ctx context.Context, me uuid.UUID, other uuid.UUID) error {
	removed, err := service.dependencies.Store.DeleteFriendship(ctx, store.DeleteFriendshipParams{Me: me, Other: other})
	if err != nil {
		return err
	}
	if removed == 0 {
		return ErrNotFound
	}
	service.notify(ctx, change{me: me, mine: RelationshipNone, other: other, theirs: RelationshipNone})
	return nil
}

// ListFriends returns the caller's friends and pending requests.
func (service *Service) ListFriends(ctx context.Context, me uuid.UUID) (Friends, error) {
	friends, err := service.dependencies.Store.ListFriends(ctx, me)
	if err != nil {
		return Friends{}, err
	}
	incoming, err := service.dependencies.Store.ListIncomingRequests(ctx, me)
	if err != nil {
		return Friends{}, err
	}
	outgoing, err := service.dependencies.Store.ListOutgoingRequests(ctx, me)
	if err != nil {
		return Friends{}, err
	}
	result := Friends{Friends: make([]Contact, 0, len(friends)), Incoming: make([]Contact, 0, len(incoming)), Outgoing: make([]Contact, 0, len(outgoing))}
	for _, row := range friends {
		result.Friends = append(result.Friends, Contact{UserID: row.ID, Username: row.Username, DisplayName: row.DisplayName, AvatarID: row.AvatarID, Since: row.Since})
	}
	for _, row := range incoming {
		result.Incoming = append(result.Incoming, Contact{UserID: row.ID, Username: row.Username, DisplayName: row.DisplayName, AvatarID: row.AvatarID, Since: row.Since})
	}
	for _, row := range outgoing {
		result.Outgoing = append(result.Outgoing, Contact{UserID: row.ID, Username: row.Username, DisplayName: row.DisplayName, AvatarID: row.AvatarID, Since: row.Since})
	}
	return result, nil
}
