package instanceadmin

import (
	"context"
	"crypto/rand"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceadmin/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	instanceconfigstore "github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers"
)

var now = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

// deletions records the servers an admin deleted.
type deletions struct {
	deleted []uuid.UUID
}

func (fake *deletions) DeleteAsAdmin(_ context.Context, serverID uuid.UUID) error {
	if serverID == uuid.Nil {
		return servers.ErrNotFound
	}
	fake.deleted = append(fake.deleted, serverID)
	return nil
}

type fixture struct {
	service  *Service
	pool     *pgxpool.Pool
	accounts *accountsstore.Queries
	servers  *deletions
	admin    uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	f := fixture{pool: pool, accounts: accountsstore.New(pool), servers: &deletions{}}
	settings := instanceconfig.NewService(instanceconfigstore.New(pool), clock.Fixed{Time: now}, "chat.example.com")
	f.service = NewService(Dependencies{Store: store.New(pool), Settings: settings, Servers: f.servers, Clock: clock.Fixed{Time: now}})
	f.admin = f.user(t, "root", true)
	return f
}

func (f fixture) user(t *testing.T, name string, admin bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, f.accounts.InsertUser(t.Context(), accountsstore.InsertUserParams{ID: id, Username: name, DisplayName: strings.ToUpper(name[:1]) + name[1:], Email: name + "@example.com", IsInstanceAdmin: admin, CreatedAt: now}))
	return id
}

func (f fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := f.pool.Exec(t.Context(), sql, args...)
	require.NoError(t, err)
}

func (f fixture) session(t *testing.T, userID uuid.UUID, lastSeen time.Time) {
	t.Helper()
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	f.exec(t, "INSERT INTO sessions (id, user_id, token_hash, device_name, created_at, last_seen_at, authenticated_at, expires_at) VALUES ($1, $2, $3, 'phone', $4, $4, $4, $5)",
		uuid.New(), userID, hash, lastSeen, now.Add(time.Hour))
}

func (f fixture) sessions(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, f.pool.QueryRow(t.Context(), "SELECT count(*) FROM sessions WHERE user_id = $1", userID).Scan(&count))
	return count
}

func usernames(users []User) []string {
	out := []string{}
	for _, user := range users {
		out = append(out, user.Username)
	}
	return out
}

func TestAdminsFindAccounts(t *testing.T) {
	f := newFixture(t)
	f.user(t, "ada", false)
	f.user(t, "grace", false)
	f.user(t, "gracie_100", false)
	users, err := f.service.Users(t.Context(), "", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"ada", "grace", "gracie_100", "root"}, usernames(users))
	users, err = f.service.Users(t.Context(), "GRAC", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"grace", "gracie_100"}, usernames(users), "username, display name or email, ignoring case")
	users, err = f.service.Users(t.Context(), "_1", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"gracie_100"}, usernames(users), "wildcards are taken literally")
	users, err = f.service.Users(t.Context(), "", "grace")
	require.NoError(t, err)
	assert.Equal(t, []string{"gracie_100", "root"}, usernames(users), "the next page")
	_, err = f.service.Users(t.Context(), strings.Repeat("x", 101), "")
	require.ErrorIs(t, err, ErrInvalid)
}

func TestSuspendingAnAccountEndsItsSessions(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	troll := f.user(t, "troll", false)
	f.session(t, troll, now)
	f.session(t, troll, now)

	require.ErrorIs(t, f.service.Suspend(ctx, f.admin, troll, strings.Repeat("x", 513)), ErrInvalid)
	require.ErrorIs(t, f.service.Suspend(ctx, f.admin, f.admin, ""), ErrSelf)
	require.ErrorIs(t, f.service.Suspend(ctx, f.admin, uuid.New(), ""), ErrNoUser)
	require.NoError(t, f.service.Suspend(ctx, f.admin, troll, "spam"))
	assert.Zero(t, f.sessions(t, troll))
	users, err := f.service.Users(ctx, "troll", "")
	require.NoError(t, err)
	require.NotNil(t, users[0].SuspendedAt)
	assert.Equal(t, "spam", users[0].SuspensionReason)

	require.NoError(t, f.service.Reinstate(ctx, f.admin, troll))
	require.ErrorIs(t, f.service.Reinstate(ctx, f.admin, troll), ErrNotSuspended)
	require.ErrorIs(t, f.service.Reinstate(ctx, f.admin, f.admin), ErrSelf)
	users, err = f.service.Users(ctx, "troll", "")
	require.NoError(t, err)
	assert.Nil(t, users[0].SuspendedAt)
	assert.Empty(t, users[0].SuspensionReason)
}

func TestTheInstanceKeepsAnAdmin(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	other := f.user(t, "deputy", false)
	require.NoError(t, f.service.SetAdmin(ctx, f.admin, other, true))
	require.ErrorIs(t, f.service.SetAdmin(ctx, f.admin, f.admin, false), ErrSelf)

	// With two, either can go; but once one is suspended, the other stays.
	require.NoError(t, f.service.Suspend(ctx, other, f.admin, "lost laptop"))
	require.ErrorIs(t, f.service.SetAdmin(ctx, uuid.New(), other, false), ErrLastAdmin)
	require.ErrorIs(t, f.service.Suspend(ctx, uuid.New(), other, ""), ErrLastAdmin)
	require.NoError(t, f.service.Reinstate(ctx, other, f.admin))
	require.NoError(t, f.service.SetAdmin(ctx, f.admin, other, false))
	require.ErrorIs(t, f.service.SetAdmin(ctx, f.admin, uuid.New(), true), ErrNoUser)
	require.NoError(t, f.service.Suspend(ctx, f.admin, other, ""), "not an admin any more")
}

func TestAdminsSeeServersAndNumbers(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	ada := f.user(t, "ada", false)
	first, second := uuid.New(), uuid.New()
	f.exec(t, "INSERT INTO servers (id, name, owner_id, created_at) VALUES ($1, 'Book club', $2, $3), ($4, 'Chess', $2, $3)", first, ada, now, second)
	f.exec(t, "INSERT INTO server_members (server_id, user_id, joined_at) VALUES ($1, $2, $3), ($1, $4, $3)", first, ada, now, f.admin)
	f.session(t, ada, now.Add(-time.Hour))
	f.session(t, f.admin, now.Add(-40*24*time.Hour))
	f.exec(t, "INSERT INTO reports (id, message_id, evidence, receipt, receipt_sig, created_at) VALUES ($1, $2, 'e', 'r', $3, $4)", uuid.New(), uuid.New(), make([]byte, 64), now)

	list, err := f.service.Servers(ctx, "", ServerCursor{})
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "Book club", list[0].Name)
	assert.Equal(t, int64(2), list[0].Members)
	assert.Equal(t, "ada", *list[0].OwnerUsername)
	next, err := f.service.Servers(ctx, "", ServerCursor{Name: list[0].Name, ID: list[0].ID.String()})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{second}, []uuid.UUID{next[0].ID})
	matched, err := f.service.Servers(ctx, "CHE", ServerCursor{})
	require.NoError(t, err)
	assert.Len(t, matched, 1)
	_, err = f.service.Servers(ctx, strings.Repeat("x", 101), ServerCursor{})
	require.ErrorIs(t, err, ErrInvalid)

	require.NoError(t, f.service.DeleteServer(ctx, second))
	assert.Equal(t, []uuid.UUID{second}, f.servers.deleted)

	stats, err := f.service.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, Stats{Users: 2, ActiveUsers: 1, Servers: 2, OpenReports: 1}, stats)
}
