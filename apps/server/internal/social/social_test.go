package social

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
	"github.com/aazam476/nexus-chat/apps/server/internal/social/store"
)

var now = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

type notice struct {
	to, about    uuid.UUID
	relationship string
}

type recorder struct {
	mutex   sync.Mutex
	notices []notice
}

func (fake *recorder) RelationshipChanged(_ context.Context, userID uuid.UUID, otherID uuid.UUID, relationship string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.notices = append(fake.notices, notice{to: userID, about: otherID, relationship: relationship})
}

func (fake *recorder) take() []notice {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	taken := fake.notices
	fake.notices = nil
	return taken
}

type limiter struct{ deny bool }

func (fake limiter) Allow(string) bool { return !fake.deny }

type sharedServer struct{ shared bool }

func (fake sharedServer) ShareAServer(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return fake.shared, nil
}

type fixture struct {
	service  *Service
	queries  *store.Queries
	notices  *recorder
	accounts *accountsstore.Queries
}

func newFixture(t *testing.T, servers ServerMembership) fixture {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	notices := &recorder{}
	queries := store.New(pool)
	service := NewService(Dependencies{Store: queries, Notifier: notices, Servers: servers, Limiter: limiter{}, Clock: clock.Fixed{Time: now}})
	return fixture{service: service, queries: queries, notices: notices, accounts: accountsstore.New(pool)}
}

func (f fixture) user(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, f.accounts.InsertUser(t.Context(), accountsstore.InsertUserParams{
		ID: id, Username: name, DisplayName: name, Email: name + "@example.com", CreatedAt: now,
	}))
	return id
}

func TestAFriendRequestBecomesAFriendship(t *testing.T) {
	f := newFixture(t, NoServers{})
	ada, grace := f.user(t, "ada"), f.user(t, "grace")

	relationship, other, err := f.service.SendRequest(t.Context(), ada, "grace")
	require.NoError(t, err)
	assert.Equal(t, RelationshipOutgoing, relationship)
	assert.Equal(t, grace, other)
	assert.Equal(t, []notice{{ada, grace, RelationshipOutgoing}, {grace, ada, RelationshipIncoming}}, f.notices.take())

	pending, err := f.service.ListFriends(t.Context(), grace)
	require.NoError(t, err)
	require.Len(t, pending.Incoming, 1)
	assert.Equal(t, "ada", pending.Incoming[0].Username)
	sent, err := f.service.ListFriends(t.Context(), ada)
	require.NoError(t, err)
	require.Len(t, sent.Outgoing, 1)

	require.NoError(t, f.service.Accept(t.Context(), grace, ada))
	assert.Equal(t, []notice{{grace, ada, RelationshipFriend}, {ada, grace, RelationshipFriend}}, f.notices.take())
	for _, me := range []uuid.UUID{ada, grace} {
		friends, err := f.service.ListFriends(t.Context(), me)
		require.NoError(t, err)
		assert.Len(t, friends.Friends, 1)
		assert.Empty(t, friends.Incoming)
		assert.Empty(t, friends.Outgoing)
	}
	require.ErrorIs(t, f.service.Accept(t.Context(), grace, ada), ErrNotFound, "the request is used up")
	_, _, err = f.service.SendRequest(t.Context(), ada, "grace")
	require.ErrorIs(t, err, ErrAlreadyFriends)

	require.NoError(t, f.service.RemoveFriend(t.Context(), ada, grace))
	require.ErrorIs(t, f.service.RemoveFriend(t.Context(), ada, grace), ErrNotFound)
	friends, err := f.service.ListFriends(t.Context(), grace)
	require.NoError(t, err)
	assert.Empty(t, friends.Friends, "removed for both")
}

func TestRequestingSomeoneWhoRequestedYouAcceptsTheirs(t *testing.T) {
	f := newFixture(t, NoServers{})
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	_, _, err := f.service.SendRequest(t.Context(), ada, "grace")
	require.NoError(t, err)
	_, _, err = f.service.SendRequest(t.Context(), ada, "grace")
	require.ErrorIs(t, err, ErrAlreadyRequested)

	relationship, _, err := f.service.SendRequest(t.Context(), grace, "ada")
	require.NoError(t, err)
	assert.Equal(t, RelationshipFriend, relationship)
}

func TestDecliningAndCancellingRequests(t *testing.T) {
	f := newFixture(t, NoServers{})
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	_, _, err := f.service.SendRequest(t.Context(), ada, "grace")
	require.NoError(t, err)

	require.NoError(t, f.service.DeleteRequest(t.Context(), grace, ada), "declined")
	require.ErrorIs(t, f.service.DeleteRequest(t.Context(), grace, ada), ErrNotFound)
	_, _, err = f.service.SendRequest(t.Context(), ada, "grace")
	require.NoError(t, err)
	require.NoError(t, f.service.DeleteRequest(t.Context(), ada, grace), "cancelled")
}

func TestRequestsToUnknownUsersYourselfOrTooOften(t *testing.T) {
	f := newFixture(t, NoServers{})
	ada := f.user(t, "ada")

	_, _, err := f.service.SendRequest(t.Context(), ada, "nobody")
	require.ErrorIs(t, err, ErrUserNotFound)
	_, _, err = f.service.SendRequest(t.Context(), ada, "ada")
	require.ErrorIs(t, err, ErrSelf)
	f.service.dependencies.Limiter = limiter{deny: true}
	_, _, err = f.service.SendRequest(t.Context(), ada, "ada")
	require.ErrorIs(t, err, ErrRateLimited)
}

func TestBlockingEndsEverythingAndStopsRequestsBothWays(t *testing.T) {
	f := newFixture(t, NoServers{})
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	_, _, err := f.service.SendRequest(t.Context(), ada, "grace")
	require.NoError(t, err)
	require.NoError(t, f.service.Accept(t.Context(), grace, ada))
	f.notices.take()

	require.NoError(t, f.service.Block(t.Context(), grace, ada))
	require.NoError(t, f.service.Block(t.Context(), grace, ada), "blocking twice changes nothing")
	assert.Equal(t, notice{grace, ada, RelationshipBlocked}, f.notices.take()[0])
	friends, err := f.service.ListFriends(t.Context(), ada)
	require.NoError(t, err)
	assert.Empty(t, friends.Friends)
	_, _, err = f.service.SendRequest(t.Context(), ada, "grace")
	require.ErrorIs(t, err, ErrNotAllowed)
	_, _, err = f.service.SendRequest(t.Context(), grace, "ada")
	require.ErrorIs(t, err, ErrNotAllowed)
	allowed, err := f.service.CanDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	assert.False(t, allowed)

	blocked, err := f.service.ListBlocked(t.Context(), grace)
	require.NoError(t, err)
	require.Len(t, blocked, 1)
	assert.Equal(t, ada, blocked[0].UserID)
	require.NoError(t, f.service.Unblock(t.Context(), grace, ada))
	require.ErrorIs(t, f.service.Unblock(t.Context(), grace, ada), ErrNotFound)
	require.ErrorIs(t, f.service.Block(t.Context(), grace, grace), ErrSelf)
	require.ErrorIs(t, f.service.Block(t.Context(), grace, uuid.New()), ErrUserNotFound)
}

func TestPrivacySettings(t *testing.T) {
	f := newFixture(t, NoServers{})
	ada, grace := f.user(t, "ada"), f.user(t, "grace")

	defaults, err := f.service.GetPrivacy(t.Context(), grace)
	require.NoError(t, err)
	assert.Equal(t, Privacy{WhoCanDirectMessage: DirectMessagesFromFriendsAndServerMembers, WhoCanFriendRequest: FriendRequestsFromEveryone}, defaults)
	allowed, err := f.service.CanDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	assert.False(t, allowed, "strangers sharing no server")

	require.NoError(t, f.service.SetPrivacy(t.Context(), grace, Privacy{WhoCanDirectMessage: DirectMessagesFromEveryone, WhoCanFriendRequest: FriendRequestsFromNobody}))
	allowed, err = f.service.CanDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	assert.True(t, allowed)
	_, _, err = f.service.SendRequest(t.Context(), ada, "grace")
	require.ErrorIs(t, err, ErrNotAllowed)

	require.ErrorIs(t, f.service.SetPrivacy(t.Context(), grace, Privacy{WhoCanDirectMessage: "anyone", WhoCanFriendRequest: FriendRequestsFromNobody}), ErrInvalidPrivacy)
	require.ErrorIs(t, f.service.SetPrivacy(t.Context(), grace, Privacy{WhoCanDirectMessage: DirectMessagesFromFriends, WhoCanFriendRequest: "anyone"}), ErrInvalidPrivacy)

	// Friends only: strangers no, friends yes.
	require.NoError(t, f.service.SetPrivacy(t.Context(), grace, Privacy{WhoCanDirectMessage: DirectMessagesFromFriends, WhoCanFriendRequest: FriendRequestsFromEveryone}))
	allowed, err = f.service.CanDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	assert.False(t, allowed)
	_, _, err = f.service.SendRequest(t.Context(), ada, "grace")
	require.NoError(t, err)
	require.NoError(t, f.service.Accept(t.Context(), grace, ada))
	allowed, err = f.service.CanDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestServerMembersSettings(t *testing.T) {
	f := newFixture(t, sharedServer{shared: true})
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	require.NoError(t, f.service.SetPrivacy(t.Context(), grace, Privacy{WhoCanDirectMessage: DirectMessagesFromFriendsAndServerMembers, WhoCanFriendRequest: FriendRequestsFromServerMembers}))

	allowed, err := f.service.CanDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	assert.True(t, allowed)
	_, _, err = f.service.SendRequest(t.Context(), ada, "grace")
	require.NoError(t, err)

	other := newFixture(t, NoServers{})
	ada, grace = other.user(t, "ada"), other.user(t, "grace")
	require.NoError(t, other.service.SetPrivacy(t.Context(), grace, Privacy{WhoCanDirectMessage: DirectMessagesFromFriends, WhoCanFriendRequest: FriendRequestsFromServerMembers}))
	_, _, err = other.service.SendRequest(t.Context(), ada, "grace")
	require.ErrorIs(t, err, ErrNotAllowed)
}

func TestNoServersMeansNoSharedServer(t *testing.T) {
	shared, err := NoServers{}.ShareAServer(t.Context(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.False(t, shared)
}

var errDown = errors.New("database down")
