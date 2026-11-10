package conversations

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

// Handler serves the conversation endpoints.
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

func avatarPath(avatarID *uuid.UUID) *string {
	if avatarID == nil {
		return nil
	}
	path := "/api/v1/avatars/" + avatarID.String()
	return &path
}

func response(conversation Conversation) api.Conversation {
	members := make([]api.UserSummary, 0, len(conversation.Members))
	for _, member := range conversation.Members {
		members = append(members, api.UserSummary{Id: member.UserID, Username: member.Username, DisplayName: member.DisplayName, AvatarUrl: avatarPath(member.AvatarID)})
	}
	return api.Conversation{
		Id: conversation.ID, Kind: api.ConversationKind(conversation.Kind), Name: conversation.Name,
		MlsGroupId: conversation.MLSGroupID, CreatedAt: conversation.CreatedAt, Members: members,
	}
}

// errorCode maps the service's errors to the codes in the contract.
func errorCode(err error) (string, bool) {
	codes := []struct {
		err  error
		code string
	}{
		{ErrNotFound, "conversation_not_found"}, {ErrUserNotFound, "user_not_found"}, {ErrSelf, "cannot_dm_self"},
		{ErrNotAllowed, "dm_not_allowed"}, {ErrGroupSize, "invalid_group_size"}, {ErrNotAGroup, "not_a_group"},
		{ErrInvalidName, "invalid_name"}, {ErrAlreadyMember, "already_member"},
	}
	for _, known := range codes {
		if errors.Is(err, known.err) {
			return known.code, true
		}
	}
	return "", false
}

// ListConversations answers GET /api/v1/conversations.
func (handler *Handler) ListConversations(ctx context.Context, _ api.ListConversationsRequestObject) (api.ListConversationsResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	list, err := handler.service.List(ctx, me)
	if err != nil {
		return nil, err
	}
	out := make(api.ListConversations200JSONResponse, 0, len(list))
	for _, conversation := range list {
		out = append(out, response(conversation))
	}
	return out, nil
}

// GetConversation answers GET /api/v1/conversations/{conversationId}.
func (handler *Handler) GetConversation(ctx context.Context, request api.GetConversationRequestObject) (api.GetConversationResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	conversation, err := handler.service.Get(ctx, me, request.ConversationId)
	if code, known := errorCode(err); known {
		return api.GetConversation404JSONResponse(apiresponse.Error(code, err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetConversation200JSONResponse(response(conversation)), nil
}
