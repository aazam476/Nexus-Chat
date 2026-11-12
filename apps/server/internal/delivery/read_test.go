package delivery

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// send has sender post one application message in the DM and returns its ID.
func (f fixture) send(t *testing.T, dmID uuid.UUID, group []byte, sender uuid.UUID) int64 {
	t.Helper()
	id, err := f.service.SendMessage(t.Context(), sender, dmID, Outgoing{Message: privateMessage(group, 0, 1)})
	require.NoError(t, err)
	return id
}

func TestReadStateCountsOthersMessagesAndOnlyMovesForward(t *testing.T) {
	f := newFixture(t)
	dm, ada, grace := f.dm(t)
	outsider := f.user(t, "mallory")

	states, err := f.service.ReadStates(t.Context(), ada)
	require.NoError(t, err)
	assert.Equal(t, []ReadState{{ConversationID: dm.ID}}, states)

	first := f.send(t, dm.ID, dm.MLSGroupID, grace)
	second := f.send(t, dm.ID, dm.MLSGroupID, grace)
	f.send(t, dm.ID, dm.MLSGroupID, ada) // her own messages are never unread
	states, err = f.service.ReadStates(t.Context(), ada)
	require.NoError(t, err)
	assert.Equal(t, []ReadState{{ConversationID: dm.ID, UnreadCount: 2}}, states)

	position, err := f.service.MarkRead(t.Context(), ada, dm.ID, first)
	require.NoError(t, err)
	assert.Equal(t, first, position)
	states, err = f.service.ReadStates(t.Context(), ada)
	require.NoError(t, err)
	assert.Equal(t, []ReadState{{ConversationID: dm.ID, LastReadID: first, UnreadCount: 1}}, states)

	// Never back, and never past the newest message.
	position, err = f.service.MarkRead(t.Context(), ada, dm.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, first, position)
	position, err = f.service.MarkRead(t.Context(), ada, dm.ID, second+1000)
	require.NoError(t, err)
	assert.Greater(t, position, second)
	assert.Less(t, position, second+1000)
	assert.Equal(t, []int64{first, first, position}, f.events.reads)

	_, err = f.service.MarkRead(t.Context(), outsider, dm.ID, first)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.MarkRead(t.Context(), ada, dm.ID, -1)
	problems, ok := errors.AsType[*validation.Error](err)
	require.True(t, ok)
	assert.Equal(t, "lastReadId", problems.Fields[0].Field)
}

// failingReads makes the read-state queries fail.
type failingReads struct {
	deliveryStore
}

var errStore = errors.New("store down")

func (failingReads) MarkRead(context.Context, store.MarkReadParams) (int64, error) {
	return 0, errStore
}

func (failingReads) ReadStates(context.Context, uuid.UUID) ([]store.ReadStatesRow, error) {
	return nil, errStore
}

func TestReadStateFailuresAreReported(t *testing.T) {
	f := newFixture(t)
	dm, ada, _ := f.dm(t)
	failing := NewService(Dependencies{Store: failingReads{f.service.dependencies.Store}, Members: f.conversations, Notifier: f.events, Clock: f.service.dependencies.Clock})

	_, err := failing.MarkRead(t.Context(), ada, dm.ID, 1)
	require.ErrorIs(t, err, errStore)
	_, err = failing.ReadStates(t.Context(), ada)
	require.ErrorIs(t, err, errStore)
}
