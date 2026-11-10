package conversations

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// OpenDirectMessage answers POST /api/v1/conversations/dms.
func (handler *Handler) OpenDirectMessage(ctx context.Context, request api.OpenDirectMessageRequestObject) (api.OpenDirectMessageResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	conversation, created, err := handler.service.OpenDirectMessage(ctx, me, request.Body.UserId)
	code, _ := errorCode(err)
	switch {
	case errors.Is(err, ErrNotAllowed):
		return api.OpenDirectMessage403JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrUserNotFound):
		return api.OpenDirectMessage404JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrSelf):
		return api.OpenDirectMessage422JSONResponse(apiresponse.Error(code, err)), nil
	case err != nil:
		return nil, err
	case created:
		return api.OpenDirectMessage201JSONResponse(response(conversation)), nil
	}
	return api.OpenDirectMessage200JSONResponse(response(conversation)), nil
}

// CreateGroupConversation answers POST /api/v1/conversations/groups.
func (handler *Handler) CreateGroupConversation(ctx context.Context, request api.CreateGroupConversationRequestObject) (api.CreateGroupConversationResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	conversation, err := handler.service.CreateGroup(ctx, me, request.Body.Name, request.Body.MemberIds)
	code, _ := errorCode(err)
	switch {
	case errors.Is(err, ErrNotAllowed):
		return api.CreateGroupConversation403JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrUserNotFound):
		return api.CreateGroupConversation404JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrGroupSize), errors.Is(err, ErrInvalidName):
		return api.CreateGroupConversation422JSONResponse(apiresponse.Error(code, err)), nil
	case err != nil:
		return nil, err
	}
	return api.CreateGroupConversation201JSONResponse(response(conversation)), nil
}

// RenameConversation answers PATCH /api/v1/conversations/{conversationId}.
func (handler *Handler) RenameConversation(ctx context.Context, request api.RenameConversationRequestObject) (api.RenameConversationResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.Rename(ctx, me, request.ConversationId, request.Body.Name)
	code, _ := errorCode(err)
	switch {
	case errors.Is(err, ErrNotFound):
		return api.RenameConversation404JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrNotAGroup), errors.Is(err, ErrInvalidName):
		return api.RenameConversation422JSONResponse(apiresponse.Error(code, err)), nil
	case err != nil:
		return nil, err
	}
	return api.RenameConversation204Response{}, nil
}

// AddConversationMember answers POST /api/v1/conversations/{conversationId}/members.
func (handler *Handler) AddConversationMember(ctx context.Context, request api.AddConversationMemberRequestObject) (api.AddConversationMemberResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.AddMember(ctx, me, request.ConversationId, request.Body.UserId)
	code, _ := errorCode(err)
	switch {
	case errors.Is(err, ErrNotAllowed):
		return api.AddConversationMember403JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrUserNotFound):
		return api.AddConversationMember404JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrAlreadyMember):
		return api.AddConversationMember409JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrNotAGroup), errors.Is(err, ErrGroupSize):
		return api.AddConversationMember422JSONResponse(apiresponse.Error(code, err)), nil
	case err != nil:
		return nil, err
	}
	return api.AddConversationMember204Response{}, nil
}

// LeaveConversation answers DELETE /api/v1/conversations/{conversationId}/members/me.
func (handler *Handler) LeaveConversation(ctx context.Context, request api.LeaveConversationRequestObject) (api.LeaveConversationResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	err = handler.service.Leave(ctx, me, request.ConversationId)
	code, _ := errorCode(err)
	switch {
	case errors.Is(err, ErrNotFound):
		return api.LeaveConversation404JSONResponse(apiresponse.Error(code, err)), nil
	case errors.Is(err, ErrNotAGroup):
		return api.LeaveConversation422JSONResponse(apiresponse.Error(code, err)), nil
	case err != nil:
		return nil, err
	}
	return api.LeaveConversation204Response{}, nil
}
