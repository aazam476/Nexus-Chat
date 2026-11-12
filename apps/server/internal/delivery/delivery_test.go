package delivery

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/conversations"
	conversationsstore "github.com/aazam476/nexus-chat/apps/server/internal/conversations/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
	"github.com/aazam476/nexus-chat/apps/server/internal/social"
	socialstore "github.com/aazam476/nexus-chat/apps/server/internal/social/store"
)

var now = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

// privateMessage builds the start of an MLS PrivateMessage: what the
// server reads, then some ciphertext-like bytes.
func privateMessage(groupID []byte, epoch uint64, contentType byte) []byte {
	message := []byte{0, 1, 0, 2, 16} // group IDs are 16 bytes
	message = append(message, groupID...)
	message = binary.BigEndian.AppendUint64(message, epoch)
	return append(message, contentType, 0, 0xde, 0xad)
}

var (
	groupInfo = []byte{0, 1, 0, 4, 9, 9} //nolint:gochecknoglobals // an MLSMessage GroupInfo header
	welcome   = []byte{0, 1, 0, 3, 7, 7} //nolint:gochecknoglobals // an MLSMessage Welcome header
)

type events struct {
	mutex  sync.Mutex
	lastID int64
	users  []uuid.UUID
	reads  []int64
}

func (fake *events) ReadChanged(_ context.Context, _ uuid.UUID, _ uuid.UUID, lastReadID int64) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.reads = append(fake.reads, lastReadID)
}

func (fake *events) MessagesArrived(_ context.Context, userIDs []uuid.UUID, _ uuid.UUID, lastID int64) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.lastID = lastID
	fake.users = userIDs
}

type quiet struct{}

func (quiet) RelationshipChanged(context.Context, uuid.UUID, uuid.UUID, string) {}
func (quiet) ConversationChanged(context.Context, []uuid.UUID, uuid.UUID)       {}
func (quiet) Allow(string) bool                                                 { return true }
func (quiet) MemberLeft(context.Context, uuid.UUID, uuid.UUID) error            { return nil }

type fixture struct {
	service       *Service
	conversations *conversations.Service
	social        *social.Service
	events        *events
	accounts      *accountsstore.Queries
	key           ed25519.PublicKey
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	system := clock.Fixed{Time: now}
	socialService := social.NewService(social.Dependencies{Store: socialstore.New(pool), Notifier: quiet{}, Servers: social.NoServers{}, Limiter: quiet{}, Clock: system})
	conversationService := conversations.NewService(conversations.Dependencies{
		Store: conversationsstore.NewWriter(pool), Policy: socialService, Notifier: quiet{}, Members: quiet{}, Clock: system,
	})
	public, private, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	recorded := &events{}
	service := NewService(Dependencies{Store: store.NewWriter(pool), Members: conversationService, Notifier: recorded, DirectiveKey: private, Clock: system})
	return fixture{service: service, conversations: conversationService, social: socialService, events: recorded, accounts: accountsstore.New(pool), key: public}
}

func (f fixture) user(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, f.accounts.InsertUser(t.Context(), accountsstore.InsertUserParams{ID: id, Username: name, DisplayName: name, Email: name + "@example.com", CreatedAt: now}))
	require.NoError(t, f.social.SetPrivacy(t.Context(), id, social.Privacy{WhoCanDirectMessage: social.DirectMessagesFromEveryone, WhoCanFriendRequest: social.FriendRequestsFromEveryone}))
	return id
}

func (f fixture) dm(t *testing.T) (conversations.Conversation, uuid.UUID, uuid.UUID) {
	t.Helper()
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	dm, _, err := f.conversations.OpenDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	return dm, ada, grace
}

func TestOneCommitPerEpochAndMessagesInOrder(t *testing.T) {
	f := newFixture(t)
	dm, ada, grace := f.dm(t)
	group := dm.MLSGroupID

	state, err := f.service.State(t.Context(), grace, dm.ID)
	require.NoError(t, err)
	assert.Equal(t, GroupState{GroupID: group, Epoch: 0}, state)

	epoch, err := f.service.SubmitCommit(t.Context(), ada, dm.ID, Commit{Commit: privateMessage(group, 0, 3), GroupInfo: groupInfo, Welcome: welcome})
	require.NoError(t, err)
	assert.Equal(t, int64(1), epoch)
	// Grace committed in the same epoch at the same time: hers loses.
	_, err = f.service.SubmitCommit(t.Context(), grace, dm.ID, Commit{Commit: privateMessage(group, 0, 3), GroupInfo: groupInfo})
	require.ErrorIs(t, err, ErrWrongEpoch)

	id, err := f.service.SendMessage(t.Context(), grace, dm.ID, Outgoing{
		Message: privateMessage(group, 1, 1), Archive: &Archive{MessageID: uuid.New(), Sealed: []byte("sealed")},
	})
	require.NoError(t, err)
	assert.Equal(t, id, f.events.lastID)
	assert.ElementsMatch(t, []uuid.UUID{ada, grace}, f.events.users)

	messages, err := f.service.Messages(t.Context(), ada, dm.ID, MessagePage{})
	require.NoError(t, err)
	require.Len(t, messages, 3)
	assert.Equal(t, []string{"commit", "welcome", "application"}, []string{messages[0].Kind, messages[1].Kind, messages[2].Kind})
	later, err := f.service.Messages(t.Context(), ada, dm.ID, MessagePage{AfterID: messages[1].ID, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, later, 1)

	archived, err := f.service.ArchivedMessages(t.Context(), ada, dm.ID, ArchivePage{Before: now.Add(time.Second)})
	require.NoError(t, err)
	require.Len(t, archived, 1)
	assert.Equal(t, []byte("sealed"), archived[0].Sealed)
	state, err = f.service.State(t.Context(), ada, dm.ID)
	require.NoError(t, err)
	assert.Equal(t, groupInfo, state.GroupInfo)
}

func TestTheStoreRefusesASecondCommitForAnEpoch(t *testing.T) {
	f := newFixture(t)
	dm, ada, _ := f.dm(t)
	writer := f.service.dependencies.Store
	_, err := f.service.State(t.Context(), ada, dm.ID)
	require.NoError(t, err)
	params := store.CommitParams{ConversationID: dm.ID, Epoch: 0, Commit: []byte{1}, GroupInfo: groupInfo, SenderID: ada, Now: now}

	_, err = writer.Commit(t.Context(), params)
	require.NoError(t, err)
	_, err = writer.Commit(t.Context(), params)
	require.ErrorIs(t, err, ErrEpochConflict)
}

func TestMessagesThatAreNotForThisGroup(t *testing.T) {
	f := newFixture(t)
	dm, ada, _ := f.dm(t)
	group := dm.MLSGroupID
	other := make([]byte, 16)

	for name, commit := range map[string]Commit{
		"another group":        {Commit: privateMessage(other, 0, 3), GroupInfo: groupInfo},
		"not a commit":         {Commit: privateMessage(group, 0, 1), GroupInfo: groupInfo},
		"garbage":              {Commit: []byte{1, 2, 3}, GroupInfo: groupInfo},
		"no group info":        {Commit: privateMessage(group, 0, 3), GroupInfo: welcome},
		"welcome is not one":   {Commit: privateMessage(group, 0, 3), GroupInfo: groupInfo, Welcome: groupInfo},
		"too large":            {Commit: append(privateMessage(group, 0, 3), make([]byte, MaximumMessageSize)...), GroupInfo: groupInfo},
		"group info too large": {Commit: privateMessage(group, 0, 3), GroupInfo: append(append([]byte{}, groupInfo...), make([]byte, MaximumMessageSize)...)},
	} {
		_, err := f.service.SubmitCommit(t.Context(), ada, dm.ID, commit)
		require.ErrorIs(t, err, ErrInvalidMessage, name)
	}
	_, err := f.service.SendMessage(t.Context(), ada, dm.ID, Outgoing{Message: privateMessage(group, 5, 1)})
	require.ErrorIs(t, err, ErrWrongEpoch)
	_, err = f.service.SendMessage(t.Context(), ada, dm.ID, Outgoing{Message: privateMessage(group, 0, 1), Archive: &Archive{MessageID: uuid.New()}})
	require.ErrorIs(t, err, ErrInvalidMessage, "empty archive copy")
	_, err = f.service.SendMessage(t.Context(), ada, dm.ID, Outgoing{Message: privateMessage(group, 0, 3)})
	require.ErrorIs(t, err, ErrInvalidMessage, "a commit sent as a message")

	repeated := uuid.New()
	_, err = f.service.SendMessage(t.Context(), ada, dm.ID, Outgoing{Message: privateMessage(group, 0, 1), Archive: &Archive{MessageID: repeated, Sealed: []byte{1}}})
	require.NoError(t, err)
	_, err = f.service.SendMessage(t.Context(), ada, dm.ID, Outgoing{Message: privateMessage(group, 0, 1), Archive: &Archive{MessageID: repeated, Sealed: []byte{1}}})
	require.ErrorIs(t, err, ErrDuplicate)
}

func TestOutsidersAndBlockedSenders(t *testing.T) {
	f := newFixture(t)
	dm, ada, grace := f.dm(t)
	outsider := f.user(t, "mallory")
	group := dm.MLSGroupID

	_, err := f.service.State(t.Context(), outsider, dm.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.Messages(t.Context(), outsider, dm.ID, MessagePage{})
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.ArchivedMessages(t.Context(), outsider, dm.ID, ArchivePage{Before: now})
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.SendMessage(t.Context(), outsider, dm.ID, Outgoing{Message: privateMessage(group, 0, 1)})
	require.ErrorIs(t, err, ErrNotAllowed)
	_, err = f.service.SubmitCommit(t.Context(), outsider, dm.ID, Commit{Commit: privateMessage(group, 0, 3), GroupInfo: groupInfo})
	require.ErrorIs(t, err, ErrNotAllowed)

	require.NoError(t, f.social.Block(t.Context(), grace, ada))
	_, err = f.service.SendMessage(t.Context(), ada, dm.ID, Outgoing{Message: privateMessage(group, 0, 1)})
	require.ErrorIs(t, err, ErrNotAllowed)
	_, err = f.service.Messages(t.Context(), ada, dm.ID, MessagePage{})
	require.NoError(t, err, "a block stops sending, not reading")
}

func TestLeavingPublishesASignedDirective(t *testing.T) {
	f := newFixture(t)
	ada, grace, alan := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "alan")
	group, err := f.conversations.CreateGroup(t.Context(), ada, nil, []uuid.UUID{grace, alan})
	require.NoError(t, err)

	require.NoError(t, f.service.MemberLeft(t.Context(), group.ID, alan))
	messages, err := f.service.Messages(t.Context(), ada, group.ID, MessagePage{})
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "directive", messages[0].Kind)
	assert.Nil(t, messages[0].SenderID)

	var signed SignedDirective
	require.NoError(t, json.Unmarshal(messages[0].Data, &signed))
	assert.True(t, VerifyDirective(f.key, signed))
	var directive Directive
	require.NoError(t, json.Unmarshal(signed.Directive, &directive))
	assert.Equal(t, Directive{Type: "remove", GroupID: group.MLSGroupID, UserID: alan, Reason: "left", IssuedAt: now}, directive)
	signed.Directive = []byte(`{"type":"remove"}`)
	assert.False(t, VerifyDirective(f.key, signed), "changed after signing")
}

func TestPageSizes(t *testing.T) {
	assert.Equal(t, int32(MaximumPage), pageSize(0))
	assert.Equal(t, int32(MaximumPage), pageSize(MaximumPage+1))
	assert.Equal(t, int32(5), pageSize(5))
}
