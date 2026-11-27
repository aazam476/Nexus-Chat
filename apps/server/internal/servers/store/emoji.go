package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
)

// Errors from AddEmoji.
var (
	ErrEmojiLimit     = errors.New("the server has as many emoji as it may")
	ErrEmojiNameTaken = errors.New("the server already has an emoji with this name")
)

// uniqueViolation is PostgreSQL's error code for a duplicate key.
const uniqueViolation = "23505"

// AddEmoji inserts emoji unless its server already has limit emoji. The
// server row is locked meanwhile, so uploads at the same moment cannot pass
// the limit together.
func (writer *Writer) AddEmoji(ctx context.Context, emoji InsertServerEmojiParams, limit int64) error {
	return database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		if _, err := queries.LockServer(ctx, emoji.ServerID); err != nil {
			return err
		}
		count, err := queries.CountServerEmoji(ctx, emoji.ServerID)
		if err != nil {
			return err
		}
		if count >= limit {
			return ErrEmojiLimit
		}
		err = queries.InsertServerEmoji(ctx, emoji)
		if pgError, ok := errors.AsType[*pgconn.PgError](err); ok && pgError.Code == uniqueViolation {
			return ErrEmojiNameTaken
		}
		return err
	})
}
