package store

import (
	"context"

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

// NewServer is a server as it starts: its row, the @everyone role, its owner
// as the first member, and its first channels.
type NewServer struct {
	Server   InsertServerParams
	Everyone InsertRoleParams
	Owner    AddServerMemberParams
	Channels []InsertChannelParams
}

// CreateServer inserts a new server and everything it starts with.
func (writer *Writer) CreateServer(ctx context.Context, server NewServer) error {
	return database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		if err := queries.InsertServer(ctx, server.Server); err != nil {
			return err
		}
		if err := queries.InsertRole(ctx, server.Everyone); err != nil {
			return err
		}
		if _, err := queries.AddServerMember(ctx, server.Owner); err != nil {
			return err
		}
		for _, channel := range server.Channels {
			if err := queries.InsertChannel(ctx, channel); err != nil {
				return err
			}
		}
		return nil
	})
}

// MoveChannels places every channel in moves together.
func (writer *Writer) MoveChannels(ctx context.Context, moves []MoveChannelParams) error {
	return database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		for _, move := range moves {
			if err := queries.MoveChannel(ctx, move); err != nil {
				return err
			}
		}
		return nil
	})
}

// OrderRoles gives the roles positions 1, 2, … in the order of ids.
func (writer *Writer) OrderRoles(ctx context.Context, ids []uuid.UUID) error {
	return database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		for index, id := range ids {
			if err := queries.SetRolePosition(ctx, SetRolePositionParams{ID: id, Position: int32(index + 1)}); err != nil { //nolint:gosec // role counts are small
				return err
			}
		}
		return nil
	})
}
