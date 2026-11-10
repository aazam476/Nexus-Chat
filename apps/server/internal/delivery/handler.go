package delivery

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// Handler serves the MLS delivery endpoints.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// newest is the cut-off when the archive is read without `before`: later
// than any copy, so the newest come first.
var newest = time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // a constant time

func caller(ctx context.Context) (uuid.UUID, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return uuid.Nil, errNoPrincipal
	}
	return principal.UserID, nil
}

func notFound(err error) api.Error {
	return apiresponse.Error("conversation_not_found", err)
}

// GetMlsGroupState answers GET /api/v1/conversations/{conversationId}/mls.
func (handler *Handler) GetMlsGroupState(ctx context.Context, request api.GetMlsGroupStateRequestObject) (api.GetMlsGroupStateResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	state, err := handler.service.State(ctx, me, request.ConversationId)
	if errors.Is(err, ErrNotFound) {
		return api.GetMlsGroupState404JSONResponse(notFound(err)), nil
	}
	if err != nil {
		return nil, err
	}
	response := api.GetMlsGroupState200JSONResponse{GroupId: state.GroupID, Epoch: state.Epoch}
	if state.GroupInfo != nil {
		response.GroupInfo = &state.GroupInfo
	}
	return response, nil
}

// SubmitMlsCommit answers POST /api/v1/conversations/{conversationId}/mls/commits.
func (handler *Handler) SubmitMlsCommit(ctx context.Context, request api.SubmitMlsCommitRequestObject) (api.SubmitMlsCommitResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	commit := Commit{Commit: request.Body.Commit, GroupInfo: request.Body.GroupInfo}
	if request.Body.Welcome != nil {
		commit.Welcome = *request.Body.Welcome
	}
	epoch, err := handler.service.SubmitCommit(ctx, me, request.ConversationId, commit)
	switch {
	case errors.Is(err, ErrNotAllowed):
		return api.SubmitMlsCommit403JSONResponse(apiresponse.Error("cannot_send", err)), nil
	case errors.Is(err, ErrNotFound):
		return api.SubmitMlsCommit404JSONResponse(notFound(err)), nil
	case errors.Is(err, ErrEpochConflict):
		return api.SubmitMlsCommit409JSONResponse(apiresponse.Error("epoch_conflict", err)), nil
	case errors.Is(err, ErrWrongEpoch):
		return api.SubmitMlsCommit409JSONResponse(apiresponse.Error("wrong_epoch", err)), nil
	case errors.Is(err, ErrInvalidMessage):
		return api.SubmitMlsCommit422JSONResponse(apiresponse.Error("invalid_mls_message", err)), nil
	case err != nil:
		return nil, err
	}
	return api.SubmitMlsCommit200JSONResponse{Epoch: epoch}, nil
}

// SendMlsMessage answers POST /api/v1/conversations/{conversationId}/mls/messages.
func (handler *Handler) SendMlsMessage(ctx context.Context, request api.SendMlsMessageRequestObject) (api.SendMlsMessageResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	outgoing := Outgoing{Message: request.Body.Message}
	if request.Body.Archive != nil {
		outgoing.Archive = &Archive{MessageID: request.Body.Archive.MessageId, Sealed: request.Body.Archive.Sealed}
	}
	id, err := handler.service.SendMessage(ctx, me, request.ConversationId, outgoing)
	switch {
	case errors.Is(err, ErrNotAllowed):
		return api.SendMlsMessage403JSONResponse(apiresponse.Error("cannot_send", err)), nil
	case errors.Is(err, ErrNotFound):
		return api.SendMlsMessage404JSONResponse(notFound(err)), nil
	case errors.Is(err, ErrWrongEpoch):
		return api.SendMlsMessage409JSONResponse(apiresponse.Error("wrong_epoch", err)), nil
	case errors.Is(err, ErrDuplicate):
		return api.SendMlsMessage409JSONResponse(apiresponse.Error("duplicate_message", err)), nil
	case errors.Is(err, ErrInvalidMessage):
		return api.SendMlsMessage422JSONResponse(apiresponse.Error("invalid_mls_message", err)), nil
	case err != nil:
		return nil, err
	}
	return api.SendMlsMessage201JSONResponse{Id: id}, nil
}
