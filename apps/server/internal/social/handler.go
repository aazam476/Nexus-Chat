package social

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// Handler serves the friends, blocks and privacy endpoints.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func caller(ctx context.Context) (uuid.UUID, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return uuid.Nil, errNoPrincipal
	}
	return principal.UserID, nil
}

func contacts(list []Contact) []api.Contact {
	out := make([]api.Contact, 0, len(list))
	for _, contact := range list {
		out = append(out, api.Contact{
			Since: contact.Since,
			User:  api.UserSummary{Id: contact.UserID, Username: contact.Username, DisplayName: contact.DisplayName, AvatarUrl: apiresponse.AvatarPath(contact.AvatarID)},
		})
	}
	return out
}

// ListFriends answers GET /api/v1/friends.
func (handler *Handler) ListFriends(ctx context.Context, _ api.ListFriendsRequestObject) (api.ListFriendsResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	friends, err := handler.service.ListFriends(ctx, me)
	if err != nil {
		return nil, err
	}
	return api.ListFriends200JSONResponse{
		Friends: contacts(friends.Friends), Incoming: contacts(friends.Incoming), Outgoing: contacts(friends.Outgoing),
	}, nil
}

// SendFriendRequest answers POST /api/v1/friends/requests.
func (handler *Handler) SendFriendRequest(ctx context.Context, request api.SendFriendRequestRequestObject) (api.SendFriendRequestResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	relationship, other, err := handler.service.SendRequest(ctx, me, request.Body.Username)
	switch {
	case errors.Is(err, ErrRateLimited):
		return api.SendFriendRequest429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case errors.Is(err, ErrUserNotFound):
		return api.SendFriendRequest404JSONResponse(apiresponse.Error("user_not_found", err)), nil
	case errors.Is(err, ErrSelf):
		return api.SendFriendRequest422JSONResponse(apiresponse.Error("cannot_befriend_self", err)), nil
	case errors.Is(err, ErrAlreadyFriends):
		return api.SendFriendRequest409JSONResponse(apiresponse.Error("already_friends", err)), nil
	case errors.Is(err, ErrAlreadyRequested):
		return api.SendFriendRequest409JSONResponse(apiresponse.Error("already_requested", err)), nil
	case errors.Is(err, ErrNotAllowed):
		return api.SendFriendRequest403JSONResponse(apiresponse.Error("friend_request_not_allowed", err)), nil
	case err != nil:
		return nil, err
	}
	return api.SendFriendRequest200JSONResponse{UserId: other, Relationship: api.Relationship(relationship)}, nil
}

// AcceptFriendRequest answers POST /api/v1/friends/requests/{userId}/accept.
func (handler *Handler) AcceptFriendRequest(ctx context.Context, request api.AcceptFriendRequestRequestObject) (api.AcceptFriendRequestResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.Accept(ctx, me, request.UserId)
	if errors.Is(err, ErrNotFound) {
		return api.AcceptFriendRequest404JSONResponse(apiresponse.Error("not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.AcceptFriendRequest204Response{}, nil
}

// DeleteFriendRequest answers DELETE /api/v1/friends/requests/{userId}.
func (handler *Handler) DeleteFriendRequest(ctx context.Context, request api.DeleteFriendRequestRequestObject) (api.DeleteFriendRequestResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.DeleteRequest(ctx, me, request.UserId)
	if errors.Is(err, ErrNotFound) {
		return api.DeleteFriendRequest404JSONResponse(apiresponse.Error("not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.DeleteFriendRequest204Response{}, nil
}

// RemoveFriend answers DELETE /api/v1/friends/{userId}.
func (handler *Handler) RemoveFriend(ctx context.Context, request api.RemoveFriendRequestObject) (api.RemoveFriendResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.RemoveFriend(ctx, me, request.UserId)
	if errors.Is(err, ErrNotFound) {
		return api.RemoveFriend404JSONResponse(apiresponse.Error("not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.RemoveFriend204Response{}, nil
}
