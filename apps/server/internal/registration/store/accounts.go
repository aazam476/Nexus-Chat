package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
)

// Errors returned by CreateAccount.
var (
	ErrInvalidInvite = errors.New("invite is unknown, used up or expired")
	ErrUsernameTaken = errors.New("username is already taken")
	ErrEmailTaken    = errors.New("email is already registered")
)

// uniqueViolation is PostgreSQL's error code for a duplicate key.
const uniqueViolation = "23505"

// NewAccount is what CreateAccount saves.
type NewAccount struct {
	User accountsstore.InsertUserParams
	// InviteHash is the hash of the invite code used, or nil if none.
	InviteHash []byte
	Now        time.Time
	// AlsoInsert, if set, saves more rows in the same transaction after
	// the user, such as an OIDC identity. Its errors are returned as is.
	AlsoInsert func(ctx context.Context, transaction pgx.Tx) error
}

// Registrar saves new accounts. It holds no SQL of its own: it combines the
// generated queries in one transaction.
type Registrar struct {
	pool *pgxpool.Pool
}

// NewRegistrar returns a Registrar using pool.
func NewRegistrar(pool *pgxpool.Pool) *Registrar {
	return &Registrar{pool: pool}
}

// CreateAccount uses up the invite (if any) and inserts the user, both or
// neither: a failed registration never consumes an invite.
func (registrar *Registrar) CreateAccount(ctx context.Context, account NewAccount) error {
	err := database.InTransaction(ctx, registrar.pool, func(transaction pgx.Tx) error {
		if account.InviteHash != nil {
			_, err := New(transaction).ConsumeInvite(ctx, ConsumeInviteParams{CodeHash: account.InviteHash, Now: account.Now})
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidInvite
			}
			if err != nil {
				return fmt.Errorf("using invite: %w", err)
			}
		}
		if err := mapDuplicate(accountsstore.New(transaction).InsertUser(ctx, account.User)); err != nil {
			return err
		}
		if account.AlsoInsert != nil {
			return account.AlsoInsert(ctx, transaction)
		}
		return nil
	})
	for _, known := range []error{ErrInvalidInvite, ErrUsernameTaken, ErrEmailTaken} {
		if errors.Is(err, known) {
			return known
		}
	}
	return err
}

// mapDuplicate turns unique-constraint violations into readable errors.
func mapDuplicate(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != uniqueViolation {
		return err
	}
	switch postgresError.ConstraintName {
	case "users_username_key":
		return ErrUsernameTaken
	case "users_email_key":
		return ErrEmailTaken
	default:
		return err
	}
}
