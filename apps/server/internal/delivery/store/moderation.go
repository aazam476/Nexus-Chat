package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
)

// ErrNoMessage means the channel has no such message.
var ErrNoMessage = errors.New("there is no such message in this channel")

// DeletedMessage is who sent a deleted channel message, and its thread.
type DeletedMessage = DeleteChannelArchiveRow

// DeleteChannelMessage deletes a channel message's archive copy and live
// copies, and uncounts it from its thread.
func (writer *Writer) DeleteChannelMessage(ctx context.Context, params DeleteChannelArchiveParams) (DeletedMessage, error) {
	var deleted DeletedMessage
	err := database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		var err error
		deleted, err = queries.DeleteChannelArchive(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoMessage
		}
		if err != nil {
			return err
		}
		if err := queries.DeleteLiveCopies(ctx, DeleteLiveCopiesParams{ConversationID: params.ConversationID, MessageID: &params.MessageID}); err != nil {
			return err
		}
		if deleted.ThreadRootID == nil || params.ChannelID == nil {
			return nil
		}
		return queries.UncountThreadReply(ctx, UncountThreadReplyParams{ChannelID: *params.ChannelID, RootMessageID: *deleted.ThreadRootID})
	})
	return deleted, err
}

// Purge deletes everything userID sent to the groups since a time, and
// recounts their threads. It returns how many messages went.
func (writer *Writer) Purge(ctx context.Context, conversationIDs []uuid.UUID, userID uuid.UUID, since time.Time) (int64, error) {
	var purged int64
	err := database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		var err error
		purged, err = queries.PurgeArchive(ctx, PurgeArchiveParams{ConversationIds: conversationIDs, UserID: &userID, Since: since})
		if err != nil {
			return err
		}
		if _, err := queries.PurgeLiveCopies(ctx, PurgeLiveCopiesParams{ConversationIds: conversationIDs, UserID: &userID, Since: since}); err != nil {
			return err
		}
		return queries.RecountThreads(ctx, conversationIDs)
	})
	return purged, err
}
