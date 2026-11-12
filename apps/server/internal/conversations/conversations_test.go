package conversations

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/conversations/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
	"github.com/aazam476/nexus-chat/apps/server/internal/social"
	socialstore "github.com/aazam476/nexus-chat/apps/server/internal/social/store"
)

var now = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

type recorder struct {
	mutex   sync.Mutex
	changed []uuid.UUID
	left    []uuid.UUID
}

func (fake *recorder) ConversationChanged(_ context.Context, userIDs []uuid.UUID, _ uuid.UUID) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.changed = append(fake.changed, userIDs...)
}

func (fake *recorder) MemberLeft(_ context.Context, _ uuid.UUID, userID uuid.UUID) error {
	fake.left = append(fake.left, userID)
	return nil
}

type allowAll struct{}

func (allowAll) Allow(string) bool { return true }

type fixture struct {
	service  *Service
	social   *social.Service
	events   *recorder
	accounts *accountsstore.Queries
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	events := &recorder{}
	socialService := social.NewService(social.Dependencies{
		Store: socialstore.New(pool), Notifier: discard{}, Servers: social.NoServers{}, Limiter: allowAll{}, Clock: clock.Fixed{Time: now},
	})
	service := NewService(Dependencies{Store: store.NewWriter(pool), Policy: socialService, Notifier: events, Members: events, Clock: clock.Fixed{Time: now}})
	return fixture{service: service, social: socialService, events: events, accounts: accountsstore.New(pool)}
}

type discard struct{}

func (discard) RelationshipChanged(context.Context, uuid.UUID, uuid.UUID, string) {}

// user makes a user who accepts DMs from everyone.
func (f fixture) user(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, f.accounts.InsertUser(t.Context(), accountsstore.InsertUserParams{ID: id, Username: name, DisplayName: name, Email: name + "@example.com", CreatedAt: now}))
	require.NoError(t, f.social.SetPrivacy(t.Context(), id, social.Privacy{WhoCanDirectMessage: social.DirectMessagesFromEveryone, WhoCanFriendRequest: social.FriendRequestsFromEveryone}))
	return id
}

func TestAPairHasOneDirectMessage(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")

	first, created, err := f.service.OpenDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, KindDirectMessage, first.Kind)
	assert.Len(t, first.MLSGroupID, 16)
	assert.Len(t, first.Members, 2)
	again, created, err := f.service.OpenDirectMessage(t.Context(), grace, ada)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, first.ID, again.ID)

	list, err := f.service.List(t.Context(), grace)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.ElementsMatch(t, []uuid.UUID{ada, grace}, f.events.changed)
}

func TestDirectMessagesFollowPrivacyAndBlocks(t *testing.T) {
	f := newFixture(t)
	ada, grace, mallory := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "mallory")

	_, _, err := f.service.OpenDirectMessage(t.Context(), ada, ada)
	require.ErrorIs(t, err, ErrSelf)
	_, _, err = f.service.OpenDirectMessage(t.Context(), ada, uuid.New())
	require.ErrorIs(t, err, ErrUserNotFound)
	require.NoError(t, f.social.SetPrivacy(t.Context(), grace, social.Privacy{WhoCanDirectMessage: social.DirectMessagesFromFriends, WhoCanFriendRequest: social.FriendRequestsFromEveryone}))
	_, _, err = f.service.OpenDirectMessage(t.Context(), ada, grace)
	require.ErrorIs(t, err, ErrNotAllowed)

	// An existing DM stays readable after a block, but nobody can send.
	dm, _, err := f.service.OpenDirectMessage(t.Context(), ada, mallory)
	require.NoError(t, err)
	maySend, err := f.service.MaySend(t.Context(), dm.ID, ada)
	require.NoError(t, err)
	assert.True(t, maySend)
	require.NoError(t, f.social.Block(t.Context(), mallory, ada))
	_, _, err = f.service.OpenDirectMessage(t.Context(), ada, mallory)
	require.NoError(t, err)
	maySend, err = f.service.MaySend(t.Context(), dm.ID, ada)
	require.NoError(t, err)
	assert.False(t, maySend)
	maySend, err = f.service.MaySend(t.Context(), dm.ID, grace)
	require.NoError(t, err)
	assert.False(t, maySend, "not a member")
	_, err = f.service.TypingRecipients(t.Context(), dm.ID, ada)
	require.ErrorIs(t, err, ErrNotFound, "blocked")
	_, err = f.service.TypingRecipients(t.Context(), dm.ID, grace)
	require.ErrorIs(t, err, ErrNotFound, "not a member")
}

func TestGroupDirectMessages(t *testing.T) {
	f := newFixture(t)
	ada, grace, alan, linus := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "alan"), f.user(t, "linus")
	name := "Compilers"

	_, err := f.service.CreateGroup(t.Context(), ada, nil, []uuid.UUID{grace, ada, grace})
	require.ErrorIs(t, err, ErrGroupSize, "only two distinct members")
	empty := ""
	_, err = f.service.CreateGroup(t.Context(), ada, &empty, []uuid.UUID{grace, alan})
	require.ErrorIs(t, err, ErrInvalidName)
	_, err = f.service.CreateGroup(t.Context(), ada, nil, []uuid.UUID{grace, uuid.New()})
	require.ErrorIs(t, err, ErrUserNotFound)

	group, err := f.service.CreateGroup(t.Context(), ada, &name, []uuid.UUID{grace, alan})
	require.NoError(t, err)
	assert.Equal(t, KindGroup, group.Kind)
	assert.Equal(t, &name, group.Name)
	assert.Len(t, group.Members, 3)
	maySend, err := f.service.MaySend(t.Context(), group.ID, grace)
	require.NoError(t, err)
	assert.True(t, maySend)
	recipients, err := f.service.TypingRecipients(t.Context(), group.ID, grace)
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{ada, alan}, recipients)

	require.NoError(t, f.service.AddMember(t.Context(), grace, group.ID, linus))
	require.ErrorIs(t, f.service.AddMember(t.Context(), grace, group.ID, linus), ErrAlreadyMember)
	renamed := "Compilers 2"
	require.NoError(t, f.service.Rename(t.Context(), alan, group.ID, &renamed))
	got, err := f.service.Get(t.Context(), linus, group.ID)
	require.NoError(t, err)
	assert.Equal(t, &renamed, got.Name)
	assert.Len(t, got.Members, 4)

	require.NoError(t, f.service.Leave(t.Context(), alan, group.ID))
	assert.Equal(t, []uuid.UUID{alan}, f.events.left)
	_, err = f.service.Get(t.Context(), alan, group.ID)
	require.ErrorIs(t, err, ErrNotFound, "gone for the one who left")
	require.ErrorIs(t, f.service.Leave(t.Context(), alan, group.ID), ErrNotFound)
}

func TestGroupLimitsAndDirectMessageRestrictions(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	dm, _, err := f.service.OpenDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)

	require.ErrorIs(t, f.service.Leave(t.Context(), ada, dm.ID), ErrNotAGroup)
	name := "x"
	require.ErrorIs(t, f.service.Rename(t.Context(), ada, dm.ID, &name), ErrNotAGroup)
	require.ErrorIs(t, f.service.AddMember(t.Context(), ada, dm.ID, grace), ErrNotAGroup)
	tooLong := string(make([]rune, 101))
	require.ErrorIs(t, f.service.Rename(t.Context(), ada, dm.ID, &tooLong), ErrInvalidName)

	members := make([]uuid.UUID, 0, DefaultMaximumGroupMembers)
	for index := range DefaultMaximumGroupMembers - 1 {
		members = append(members, f.user(t, "member"+string(rune('a'+index))))
	}
	full, err := f.service.CreateGroup(t.Context(), ada, nil, members)
	require.NoError(t, err)
	require.ErrorIs(t, f.service.AddMember(t.Context(), ada, full.ID, grace), ErrGroupSize)
	_, err = f.service.CreateGroup(t.Context(), ada, nil, append(members, grace))
	require.ErrorIs(t, err, ErrGroupSize)
	_, err = f.service.Lookup(t.Context(), uuid.New())
	require.ErrorIs(t, err, ErrNotFound)
}

func TestTheInstanceSetsTheGroupLimit(t *testing.T) {
	f := newFixture(t)
	small := NewService(Dependencies{
		Store: f.service.dependencies.Store, Policy: f.social, Notifier: f.events, Members: f.events,
		Clock: clock.Fixed{Time: now}, MaximumGroupMembers: 3,
	})
	ada, grace, alan, linus := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "alan"), f.user(t, "linus")

	_, err := small.CreateGroup(t.Context(), ada, nil, []uuid.UUID{grace, alan, linus})
	require.ErrorIs(t, err, ErrGroupSize)
	group, err := small.CreateGroup(t.Context(), ada, nil, []uuid.UUID{grace, alan})
	require.NoError(t, err)
	require.ErrorIs(t, small.AddMember(t.Context(), ada, group.ID, linus), ErrGroupSize)
}
