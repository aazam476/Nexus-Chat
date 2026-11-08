package registration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

type fakeInviteStore struct {
	inserted []store.InsertInviteParams
	deleted  int64
	err      error
}

func (fake *fakeInviteStore) InsertInvite(_ context.Context, arg store.InsertInviteParams) error {
	fake.inserted = append(fake.inserted, arg)
	return fake.err
}

func (fake *fakeInviteStore) ListInvites(context.Context) ([]store.ListInvitesRow, error) {
	return []store.ListInvitesRow{{ID: uuid.New()}}, fake.err
}

func (fake *fakeInviteStore) DeleteInvite(context.Context, uuid.UUID) (int64, error) {
	return fake.deleted, fake.err
}

func TestCreateInvite(t *testing.T) {
	invites := &fakeInviteStore{}
	expires := now.Add(24 * time.Hour)
	admin := uuid.New()

	code, id, err := NewInvites(invites, clock.Fixed{Time: now}).Create(t.Context(), admin, InviteOptions{MaxUses: 3, ExpiresAt: &expires})

	require.NoError(t, err)
	assert.Regexp(t, `^([A-Z2-7]{4}-){6}[A-Z2-7]{2}$`, code)
	require.Len(t, invites.inserted, 1)
	saved := invites.inserted[0]
	assert.Equal(t, id, saved.ID)
	expected := HashInviteCode(code)
	assert.Equal(t, expected[:], saved.CodeHash)
	assert.Equal(t, int32(3), *saved.MaxUses)
	assert.Equal(t, admin, *saved.CreatedBy)
}

func TestUnlimitedInvite(t *testing.T) {
	invites := &fakeInviteStore{}

	_, _, err := NewInvites(invites, clock.Fixed{Time: now}).Create(t.Context(), uuid.New(), InviteOptions{})

	require.NoError(t, err)
	assert.Nil(t, invites.inserted[0].MaxUses)
	assert.Nil(t, invites.inserted[0].ExpiresAt)
}

func TestListAndDeleteInvites(t *testing.T) {
	invites := &fakeInviteStore{deleted: 1}
	service := NewInvites(invites, clock.Fixed{Time: now})

	rows, err := service.List(t.Context())
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	deleted, err := service.Delete(t.Context(), uuid.New())
	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestInviteStoreFailures(t *testing.T) {
	failure := errors.New("database down")
	service := NewInvites(&fakeInviteStore{err: failure}, clock.Fixed{Time: now})

	_, _, err := service.Create(t.Context(), uuid.New(), InviteOptions{})
	require.ErrorIs(t, err, failure)
	_, err = service.List(t.Context())
	require.ErrorIs(t, err, failure)
	_, err = service.Delete(t.Context(), uuid.New())
	assert.ErrorIs(t, err, failure)
}
