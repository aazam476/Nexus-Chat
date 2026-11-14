package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
)

// Errors the writer reports.
var (
	// ErrEpochConflict means another commit for that epoch was accepted first.
	ErrEpochConflict = errors.New("a commit for this epoch was already accepted")
	// ErrDuplicateMessage means an archive copy with that message ID exists.
	ErrDuplicateMessage = errors.New("a message with this ID was already sent")
)

// isDuplicateArchive reports whether err is the archive's primary key
// rejecting a repeated message ID (PostgreSQL code 23505).
func isDuplicateArchive(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "archived_messages_pkey"
}

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

// CommitParams is an accepted commit and what travels with it.
type CommitParams struct {
	ConversationID uuid.UUID
	// Epoch is the epoch the commit was made in; the group moves to Epoch+1.
	Epoch     int64
	Commit    []byte
	GroupInfo []byte
	// Welcome is nil when the commit adds nobody.
	Welcome  []byte
	SenderID uuid.UUID
	Now      time.Time
}

// Commit advances the epoch and stores the commit (and welcome) together,
// or returns ErrEpochConflict and changes nothing. It returns the ID of
// the last message stored.
func (writer *Writer) Commit(ctx context.Context, params CommitParams) (int64, error) {
	var lastID int64
	err := database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		_, err := queries.AdvanceEpoch(ctx, AdvanceEpochParams{
			ConversationID: params.ConversationID, ExpectedEpoch: params.Epoch, GroupInfo: params.GroupInfo, Now: params.Now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEpochConflict
		}
		if err != nil {
			return err
		}
		lastID, err = queries.InsertMessage(ctx, InsertMessageParams{
			ConversationID: params.ConversationID, Kind: "commit", Epoch: params.Epoch, SenderID: &params.SenderID, Data: params.Commit, CreatedAt: params.Now,
		})
		if err != nil || params.Welcome == nil {
			return err
		}
		lastID, err = queries.InsertMessage(ctx, InsertMessageParams{
			ConversationID: params.ConversationID, Kind: "welcome", Epoch: params.Epoch + 1, SenderID: &params.SenderID, Data: params.Welcome, CreatedAt: params.Now,
		})
		return err
	})
	return lastID, err
}

// SendParams is an application message's live copy and, when given, its archive copy.
type SendParams struct {
	ConversationID uuid.UUID
	Epoch          int64
	Message        []byte
	ArchiveID      *uuid.UUID
	Archive        []byte
	SenderID       uuid.UUID
	Now            time.Time
	// ChannelID is set for a server channel's message in its audience's group.
	ChannelID *uuid.UUID
}

// Send stores both copies of an application message together.
func (writer *Writer) Send(ctx context.Context, params SendParams) (int64, error) {
	var id int64
	err := database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		var err error
		id, err = queries.InsertMessage(ctx, InsertMessageParams{
			ConversationID: params.ConversationID, Kind: "application", Epoch: params.Epoch, SenderID: &params.SenderID, Data: params.Message, CreatedAt: params.Now,
			ChannelID: params.ChannelID,
		})
		if err != nil || params.ArchiveID == nil {
			return err
		}
		err = queries.InsertArchive(ctx, InsertArchiveParams{
			ConversationID: params.ConversationID, MessageID: *params.ArchiveID, Epoch: params.Epoch, SenderID: &params.SenderID, Sealed: params.Archive, CreatedAt: params.Now,
			ChannelID: params.ChannelID,
		})
		if isDuplicateArchive(err) {
			return ErrDuplicateMessage
		}
		return err
	})
	return id, err
}
