package social

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/social/store"
)

// Who may start a DM (spec §4.2).
const (
	DirectMessagesFromEveryone                = "everyone"
	DirectMessagesFromFriends                 = "friends"
	DirectMessagesFromFriendsAndServerMembers = "friends_and_server_members"
)

// Who may send a friend request.
const (
	FriendRequestsFromEveryone      = "everyone"
	FriendRequestsFromServerMembers = "server_members"
	FriendRequestsFromNobody        = "nobody"
)

// ErrInvalidPrivacy means a setting is not one of the choices above.
var ErrInvalidPrivacy = errors.New("unknown privacy choice")

// Privacy is a user's privacy settings.
type Privacy struct {
	WhoCanDirectMessage string
	WhoCanFriendRequest string
}

// GetPrivacy returns the caller's settings (the defaults if never changed).
func (service *Service) GetPrivacy(ctx context.Context, me uuid.UUID) (Privacy, error) {
	row, err := service.dependencies.Store.GetPrivacy(ctx, me)
	if err != nil {
		return Privacy{}, err
	}
	return Privacy{WhoCanDirectMessage: row.WhoCanDm, WhoCanFriendRequest: row.WhoCanFriendRequest}, nil
}

// SetPrivacy replaces the caller's settings.
func (service *Service) SetPrivacy(ctx context.Context, me uuid.UUID, privacy Privacy) error {
	switch privacy.WhoCanDirectMessage {
	case DirectMessagesFromEveryone, DirectMessagesFromFriends, DirectMessagesFromFriendsAndServerMembers:
	default:
		return ErrInvalidPrivacy
	}
	switch privacy.WhoCanFriendRequest {
	case FriendRequestsFromEveryone, FriendRequestsFromServerMembers, FriendRequestsFromNobody:
	default:
		return ErrInvalidPrivacy
	}
	return service.dependencies.Store.PutPrivacy(ctx, store.PutPrivacyParams{
		UserID: me, WhoCanDm: privacy.WhoCanDirectMessage, WhoCanFriendRequest: privacy.WhoCanFriendRequest,
	})
}

// CanDirectMessage reports whether sender may start a DM with recipient
// or add them to a group DM: neither blocked the other, and the
// recipient's setting allows the sender.
func (service *Service) CanDirectMessage(ctx context.Context, sender uuid.UUID, recipient uuid.UUID) (bool, error) {
	current, err := service.relationship(ctx, sender, recipient)
	if err != nil {
		return false, err
	}
	if current.IBlocked || current.BlockedMe {
		return false, nil
	}
	privacy, err := service.dependencies.Store.GetPrivacy(ctx, recipient)
	if err != nil {
		return false, err
	}
	switch privacy.WhoCanDm {
	case DirectMessagesFromEveryone:
		return true, nil
	case DirectMessagesFromFriendsAndServerMembers:
		if current.Friends {
			return true, nil
		}
		return service.dependencies.Servers.ShareAServer(ctx, sender, recipient)
	default:
		return current.Friends, nil
	}
}
