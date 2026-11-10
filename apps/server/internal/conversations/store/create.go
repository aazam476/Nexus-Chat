package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
)

// Writer makes changes that span several statements, each in one
// transaction, on top of the generated queries.
type Writer struct {
	*Queries
	pool *pgxpool.Pool
}

// NewWriter returns a Writer on pool.
func NewWriter(pool *pgxpool.Pool) *Writer {
	return &Writer{Queries: New(pool), pool: pool}
}

// Create inserts a conversation and its first members together.
func (writer *Writer) Create(ctx context.Context, conversation InsertConversationParams, memberIDs []uuid.UUID, now time.Time) error {
	return database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		if err := queries.InsertConversation(ctx, conversation); err != nil {
			return err
		}
		for _, userID := range memberIDs {
			if err := queries.AddMember(ctx, AddMemberParams{ConversationID: conversation.ID, UserID: userID, JoinedAt: now}); err != nil {
				return err
			}
		}
		return nil
	})
}
