package delivery

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// Read state (spec §4.5): how far each member has read each conversation,
// by delivery ID, so all of their devices show the same unread markers.

// errNegativeReadPosition explains a negative lastReadId.
var errNegativeReadPosition = errors.New("must not be negative")

// ReadState is how far a member has read a conversation.
type ReadState struct {
	ConversationID uuid.UUID
	LastReadID     int64
	// UnreadCount counts application messages from others after
	// LastReadID, up to 1000.
	UnreadCount int64
}

type readStore interface {
	MarkRead(ctx context.Context, params store.MarkReadParams) (int64, error)
	ReadStates(ctx context.Context, userID uuid.UUID) ([]store.ReadStatesRow, error)
}

// ReadNotifier tells a user's devices that their read position moved.
type ReadNotifier interface {
	ReadChanged(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID, lastReadID int64)
}

// MarkRead moves the caller's read position forward to lastReadID (never
// back, never past the newest message) and returns where it now is. A
// negative lastReadID is a *validation.Error.
func (service *Service) MarkRead(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, lastReadID int64) (int64, error) {
	if lastReadID < 0 {
		var problems validation.Errors
		problems.Add("lastReadId", errNegativeReadPosition)
		return 0, problems.Err()
	}
	isMember, err := service.dependencies.Members.IsMember(ctx, conversationID, me)
	if err != nil {
		return 0, err
	}
	if !isMember {
		return 0, ErrNotFound
	}
	position, err := service.dependencies.Store.MarkRead(ctx, store.MarkReadParams{
		ConversationID: conversationID, UserID: me, LastReadID: lastReadID, Now: service.dependencies.Clock.Now(),
	})
	if err != nil {
		return 0, err
	}
	service.dependencies.Notifier.ReadChanged(ctx, me, conversationID, position)
	return position, nil
}

// ReadStates returns the caller's read state in each of their conversations.
func (service *Service) ReadStates(ctx context.Context, me uuid.UUID) ([]ReadState, error) {
	rows, err := service.dependencies.Store.ReadStates(ctx, me)
	if err != nil {
		return nil, err
	}
	states := make([]ReadState, 0, len(rows))
	for _, row := range rows {
		states = append(states, ReadState(row))
	}
	return states, nil
}

// MarkConversationRead answers PUT /api/v1/conversations/{conversationId}/read.
func (handler *Handler) MarkConversationRead(ctx context.Context, request api.MarkConversationReadRequestObject) (api.MarkConversationReadResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	position, err := handler.service.MarkRead(ctx, me, request.ConversationId, request.Body.LastReadId)
	if errors.Is(err, ErrNotFound) {
		return api.MarkConversationRead404JSONResponse(notFound(err)), nil
	}
	if problems, ok := errors.AsType[*validation.Error](err); ok {
		return api.MarkConversationRead422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	}
	if err != nil {
		return nil, err
	}
	state := api.ReadState{ConversationId: request.ConversationId, LastReadId: position}
	states, err := handler.service.ReadStates(ctx, me)
	if err != nil {
		return nil, err
	}
	for _, each := range states {
		if each.ConversationID == request.ConversationId {
			state.UnreadCount = each.UnreadCount
		}
	}
	return api.MarkConversationRead200JSONResponse(state), nil
}

// ListReadStates answers GET /api/v1/read-state.
func (handler *Handler) ListReadStates(ctx context.Context, _ api.ListReadStatesRequestObject) (api.ListReadStatesResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	states, err := handler.service.ReadStates(ctx, me)
	if err != nil {
		return nil, err
	}
	out := make(api.ListReadStates200JSONResponse, 0, len(states))
	for _, state := range states {
		out = append(out, api.ReadState{ConversationId: state.ConversationID, LastReadId: state.LastReadID, UnreadCount: state.UnreadCount})
	}
	return out, nil
}
