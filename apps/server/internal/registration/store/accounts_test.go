package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
)

var now = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func newRegistrar(t *testing.T) *Registrar {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	return NewRegistrar(pool)
}

func user(name string) accountsstore.InsertUserParams {
	hash := "hash"
	return accountsstore.InsertUserParams{ID: uuid.New(), Username: name, DisplayName: name, Email: name + "@example.com", PasswordHash: &hash, CreatedAt: now}
}

// inviteSpec describes a test invite.
type inviteSpec struct {
	code    string
	maxUses int32
	expires *time.Time
}

func addInvite(t *testing.T, registrar *Registrar, spec inviteSpec) []byte {
	t.Helper()
	hash := sha256.Sum256([]byte(spec.code))
	err := New(registrar.pool).InsertInvite(context.Background(), InsertInviteParams{
		ID: uuid.New(), CodeHash: hash[:], MaxUses: &spec.maxUses, ExpiresAt: spec.expires, CreatedAt: now,
	})
	require.NoError(t, err)
	return hash[:]
}

func TestInvitesAreUsedUpExactly(t *testing.T) {
	registrar := newRegistrar(t)
	invite := addInvite(t, registrar, inviteSpec{code: "CODE", maxUses: 2})
	ctx := context.Background()

	require.NoError(t, registrar.CreateAccount(ctx, NewAccount{User: user("first"), InviteHash: invite, Now: now}))
	require.NoError(t, registrar.CreateAccount(ctx, NewAccount{User: user("second"), InviteHash: invite, Now: now}))
	err := registrar.CreateAccount(ctx, NewAccount{User: user("third"), InviteHash: invite, Now: now})

	require.ErrorIs(t, err, ErrInvalidInvite)
	_, err = accountsstore.New(registrar.pool).GetUserByUsername(ctx, "third")
	assert.Error(t, err, "no account without a valid invite")
}

func TestExpiredAndUnknownInvites(t *testing.T) {
	registrar := newRegistrar(t)
	past := now.Add(-time.Minute)
	expired := addInvite(t, registrar, inviteSpec{code: "OLD", maxUses: 5, expires: &past})
	unknown := sha256.Sum256([]byte("NOPE"))

	require.ErrorIs(t, registrar.CreateAccount(context.Background(), NewAccount{User: user("a"), InviteHash: expired, Now: now}), ErrInvalidInvite)
	assert.ErrorIs(t, registrar.CreateAccount(context.Background(), NewAccount{User: user("b"), InviteHash: unknown[:], Now: now}), ErrInvalidInvite)
}

func TestDuplicatesAreReportedAndKeepTheInvite(t *testing.T) {
	registrar := newRegistrar(t)
	ctx := context.Background()
	require.NoError(t, registrar.CreateAccount(ctx, NewAccount{User: user("alice"), Now: now}))
	invite := addInvite(t, registrar, inviteSpec{code: "ONCE", maxUses: 1})

	sameName := user("alice")
	sameName.Email = "other@example.com"
	require.ErrorIs(t, registrar.CreateAccount(ctx, NewAccount{User: sameName, InviteHash: invite, Now: now}), ErrUsernameTaken)
	sameEmail := user("alicia")
	sameEmail.Email = "alice@example.com"
	require.ErrorIs(t, registrar.CreateAccount(ctx, NewAccount{User: sameEmail, InviteHash: invite, Now: now}), ErrEmailTaken)

	assert.NoError(t, registrar.CreateAccount(ctx, NewAccount{User: user("carol"), InviteHash: invite, Now: now}),
		"the failed attempts rolled back, so the single-use invite still works")
}

func TestOtherDatabaseErrorsPassThrough(t *testing.T) {
	registrar := newRegistrar(t)
	invalid := user("UPPER") // breaks the username CHECK constraint, not a duplicate

	err := registrar.CreateAccount(context.Background(), NewAccount{User: invalid, Now: now})

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUsernameTaken)
}

func TestInviteQueryFailure(t *testing.T) {
	registrar := newRegistrar(t)
	registrar.pool.Close()

	err := registrar.CreateAccount(context.Background(), NewAccount{User: user("a"), InviteHash: []byte("x"), Now: now})

	assert.Error(t, err)
}

func TestAlsoInsertRunsInTheSameTransaction(t *testing.T) {
	registrar := newRegistrar(t)
	invite := addInvite(t, registrar, inviteSpec{code: "ONCE", maxUses: 1})
	ctx := context.Background()
	failure := errors.New("identity already linked")

	err := registrar.CreateAccount(ctx, NewAccount{User: user("rolled"), InviteHash: invite, Now: now,
		AlsoInsert: func(context.Context, pgx.Tx) error { return failure }})
	require.ErrorIs(t, err, failure)
	_, err = accountsstore.New(registrar.pool).GetUserByUsername(ctx, "rolled")
	require.Error(t, err, "the user is rolled back")

	ran := false
	err = registrar.CreateAccount(ctx, NewAccount{User: user("kept"), InviteHash: invite, Now: now,
		AlsoInsert: func(context.Context, pgx.Tx) error {
			ran = true
			return nil
		}})
	require.NoError(t, err, "the failed attempt did not use up the invite")
	assert.True(t, ran)
}
