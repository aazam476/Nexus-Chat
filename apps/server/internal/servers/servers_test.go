package servers

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

var start = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

type testClock struct {
	mutex sync.Mutex
	now   time.Time
}

func (clock *testClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.now
}

func (clock *testClock) advance(by time.Duration) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.now = clock.now.Add(by)
}

type recorder struct {
	mutex   sync.Mutex
	changed []uuid.UUID
}

func (fake *recorder) ServerChanged(_ context.Context, userIDs []uuid.UUID, _ uuid.UUID) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.changed = append(fake.changed, userIDs...)
}

// removal is someone asked to leave an audience's MLS group.
type removal struct {
	audienceID uuid.UUID
	userID     uuid.UUID
}

// leaves records the removals the service asks for.
type leaves struct {
	mutex    sync.Mutex
	removals []removal
	fail     error
}

func (fake *leaves) MemberLeft(_ context.Context, audienceID uuid.UUID, userID uuid.UUID) error {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.removals = append(fake.removals, removal{audienceID: audienceID, userID: userID})
	return fake.fail
}

type fixture struct {
	service  *Service
	events   *recorder
	clock    *testClock
	accounts *accountsstore.Queries
	store    *store.Writer
	pool     *pgxpool.Pool
	leaves   *leaves
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	events, now := &recorder{}, &testClock{now: start}
	writer := store.NewWriter(pool)
	removals := &leaves{}
	service := NewService(Dependencies{Store: writer, Notifier: events, Audiences: removals, Clock: now})
	return fixture{service: service, events: events, clock: now, accounts: accountsstore.New(pool), store: writer, pool: pool, leaves: removals}
}

func (f fixture) user(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, f.accounts.InsertUser(t.Context(), accountsstore.InsertUserParams{ID: id, Username: name, DisplayName: name, Email: name + "@example.com", CreatedAt: start}))
	return id
}

// server makes a server owned by owner, with members who joined by invite.
func (f fixture) server(t *testing.T, owner uuid.UUID, members ...uuid.UUID) Details {
	t.Helper()
	details, err := f.service.Create(t.Context(), owner, "Book club", "We read.")
	require.NoError(t, err)
	invite, err := f.service.CreateInvite(t.Context(), Scope{details.Server.ID, owner}, InviteLimits{})
	require.NoError(t, err)
	for _, member := range members {
		_, err := f.service.Join(t.Context(), invite.Code, member)
		require.NoError(t, err)
	}
	return details
}

func channelNamed(t *testing.T, details Details, name string) Channel {
	t.Helper()
	for _, channel := range details.Channels {
		if channel.Name == name {
			return channel
		}
	}
	t.Fatalf("no channel %q", name)
	return Channel{}
}

func TestANewServerHasEveryoneAndStarterChannels(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	details, err := f.service.Create(t.Context(), ada, "  Book club  ", "We read.")
	require.NoError(t, err)

	assert.Equal(t, "Book club", details.Server.Name)
	assert.Equal(t, ada, details.Server.OwnerID)
	require.Len(t, details.Roles, 1)
	assert.True(t, details.Roles[0].Everyone)
	assert.Equal(t, permissions.DefaultEveryone, details.Roles[0].Permissions)
	assert.Equal(t, permissions.All, details.Permissions)
	names := []string{}
	for _, channel := range details.Channels {
		names = append(names, channel.Kind+":"+channel.Name)
	}
	assert.ElementsMatch(t, []string{"category:Text channels", "category:Voice channels", "text:general", "voice:General"}, names)
	general := channelNamed(t, details, "general")
	assert.Len(t, general.MLSGroupID, 16)
	assert.Nil(t, channelNamed(t, details, "General").MLSGroupID)

	list, err := f.service.List(t.Context(), ada)
	require.NoError(t, err)
	require.Len(t, list, 1)
	_, err = f.service.Create(t.Context(), ada, " ", "")
	require.ErrorIs(t, err, ErrInvalid)
}

func TestOutsidersSeeNothing(t *testing.T) {
	f := newFixture(t)
	ada, eve := f.user(t, "ada"), f.user(t, "eve")
	details := f.server(t, ada)
	scope := Scope{details.Server.ID, eve}
	_, err := f.service.Details(t.Context(), scope)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.Members(t.Context(), scope)
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, f.service.Leave(t.Context(), scope), ErrNotFound)
	_, err = f.service.Details(t.Context(), Scope{uuid.New(), ada})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSettingsLeavingAndDeleting(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	owner, member := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}
	limit := int64(1 << 20)
	settings := Settings{Name: "Readers", Description: "Still reading.", AttachmentMaxBytes: &limit, CompressMedia: true}

	require.ErrorIs(t, f.service.Update(t.Context(), member, settings), ErrForbidden)
	require.NoError(t, f.service.Update(t.Context(), owner, settings))
	seen, err := f.service.Details(t.Context(), member)
	require.NoError(t, err)
	assert.Equal(t, "Readers", seen.Server.Name)
	assert.Equal(t, &limit, seen.Server.AttachmentMaxBytes)
	assert.True(t, seen.Server.CompressMedia)
	assert.Contains(t, f.events.changed, grace)
	zero := int64(0)
	require.ErrorIs(t, f.service.Update(t.Context(), owner, Settings{Name: "x", AttachmentMaxBytes: &zero}), ErrInvalid)

	shares, err := f.service.ShareAServer(t.Context(), ada, grace)
	require.NoError(t, err)
	assert.True(t, shares)
	require.ErrorIs(t, f.service.Leave(t.Context(), owner), ErrOwnerLeaving)
	require.ErrorIs(t, f.service.Delete(t.Context(), member), ErrForbidden)
	require.NoError(t, f.service.Leave(t.Context(), member))
	shares, err = f.service.ShareAServer(t.Context(), ada, grace)
	require.NoError(t, err)
	assert.False(t, shares)
	require.NoError(t, f.service.Delete(t.Context(), owner))
	_, err = f.service.Details(t.Context(), owner)
	require.ErrorIs(t, err, ErrNotFound)
}
