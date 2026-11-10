package delivery

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
)

// MaximumPage is the most messages or archive copies returned at once.
const MaximumPage = 200

// ArchivedMessage is an archive copy as devices fetch it.
type ArchivedMessage struct {
	MessageID uuid.UUID
	Epoch     int64
	SenderID  *uuid.UUID
	Sealed    []byte
	CreatedAt time.Time
}

func pageSize(limit int) int32 {
	if limit <= 0 || limit > MaximumPage {
		return MaximumPage
	}
	return int32(limit) //nolint:gosec // at most MaximumPage
}

// MessagePage asks for live copies after AfterID, at most Limit of them
// (0 or too many means MaximumPage).
type MessagePage struct {
	AfterID int64
	Limit   int
}

// ArchivePage asks for archive copies made before Before.
type ArchivePage struct {
	Before time.Time
	Limit  int
}

// Messages returns live copies after page.AfterID, oldest first.
func (service *Service) Messages(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, page MessagePage) ([]Message, error) {
	if _, err := service.group(ctx, me, conversationID); err != nil {
		return nil, err
	}
	rows, err := service.dependencies.Store.MessagesAfter(ctx, store.MessagesAfterParams{ConversationID: conversationID, AfterID: page.AfterID, RowLimit: pageSize(page.Limit)})
	if err != nil {
		return nil, err
	}
	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, Message(row))
	}
	return messages, nil
}

// ArchivedMessages returns archive copies made before page.Before, newest first.
func (service *Service) ArchivedMessages(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, page ArchivePage) ([]ArchivedMessage, error) {
	if _, err := service.group(ctx, me, conversationID); err != nil {
		return nil, err
	}
	rows, err := service.dependencies.Store.ArchiveBefore(ctx, store.ArchiveBeforeParams{ConversationID: conversationID, Before: page.Before, RowLimit: pageSize(page.Limit)})
	if err != nil {
		return nil, err
	}
	archived := make([]ArchivedMessage, 0, len(rows))
	for _, row := range rows {
		archived = append(archived, ArchivedMessage(row))
	}
	return archived, nil
}
