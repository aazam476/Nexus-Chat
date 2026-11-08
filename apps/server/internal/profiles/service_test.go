package profiles

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
)

func pointer(text string) *string { return &text }

func fieldOf(t *testing.T, err error) string {
	t.Helper()
	var problems *validation.Error
	require.ErrorAs(t, err, &problems)
	require.Len(t, problems.Fields, 1)
	return problems.Fields[0].Field
}

func TestUpdateProfileChangesOnlyWhatIsSent(t *testing.T) {
	fixture := newFixture(t)

	user, err := fixture.service.UpdateProfile(t.Context(), fixture.user.ID, ProfileUpdate{Bio: pointer("  Mathematician  ")})
	require.NoError(t, err)
	assert.Equal(t, "Mathematician", user.Bio)
	assert.Equal(t, "Ada", user.DisplayName)

	user, err = fixture.service.UpdateProfile(t.Context(), fixture.user.ID, ProfileUpdate{DisplayName: pointer("Ada L.")})
	require.NoError(t, err)
	assert.Equal(t, "Ada L.", user.DisplayName)
	assert.Equal(t, "Mathematician", user.Bio)
	assert.Len(t, fixture.listener.changes, 2, "every change is announced")
}

func TestUpdateProfileValidates(t *testing.T) {
	fixture := newFixture(t)
	for update, field := range map[*ProfileUpdate]string{
		{DisplayName: pointer("")}:                      "displayName",
		{DisplayName: pointer(" padded")}:               "displayName",
		{Bio: pointer(strings.Repeat("b", 501))}:        "bio",
		{Bio: pointer("\xff")}:                          "bio",
		{DisplayName: pointer(strings.Repeat("d", 33))}: "displayName",
	} {
		_, err := fixture.service.UpdateProfile(t.Context(), fixture.user.ID, *update)
		assert.Equal(t, field, fieldOf(t, err))
	}
	assert.Empty(t, fixture.listener.changes)
}

func TestUpdateProfileErrors(t *testing.T) {
	fixture := newFixture(t)
	_, err := fixture.service.UpdateProfile(t.Context(), uuid.New(), ProfileUpdate{})
	require.ErrorIs(t, err, ErrUserNotFound)

	fixture.users.failOn = "UpdateProfile"
	_, err = fixture.service.UpdateProfile(t.Context(), fixture.user.ID, ProfileUpdate{})
	require.ErrorIs(t, err, errFailure)

	fixture.users.failOn = "GetUserByID"
	_, err = fixture.service.Get(t.Context(), fixture.user.ID)
	assert.ErrorIs(t, err, errFailure)
}

func TestSetStatus(t *testing.T) {
	fixture := newFixture(t)
	later := testNow.Add(time.Hour)

	user, err := fixture.service.SetStatus(t.Context(), fixture.user.ID, StatusUpdate{Status: "dnd", CustomText: "  In a   meeting ", ExpiresAt: &later})
	require.NoError(t, err)
	assert.Equal(t, "dnd", user.Status)
	assert.Equal(t, "In a meeting", *user.CustomStatusText)
	assert.Equal(t, later, *user.CustomStatusExpiresAt)

	user, err = fixture.service.SetStatus(t.Context(), fixture.user.ID, StatusUpdate{Status: "invisible"})
	require.NoError(t, err)
	assert.Nil(t, user.CustomStatusText, "leaving the text out removes it")
	assert.Nil(t, user.CustomStatusExpiresAt)
	assert.Len(t, fixture.listener.changes, 2)
}

func TestSetStatusValidates(t *testing.T) {
	fixture := newFixture(t)
	past, later := testNow.Add(-time.Minute), testNow.Add(time.Minute)
	tests := map[string]struct {
		update StatusUpdate
		field  string
	}{
		"unknown status":      {StatusUpdate{Status: "offline"}, "status"},
		"expiry without text": {StatusUpdate{Status: "online", ExpiresAt: &later}, "customStatus.text"},
		"text too long":       {StatusUpdate{Status: "online", CustomText: strings.Repeat("t", 129)}, "customStatus.text"},
		"control character":   {StatusUpdate{Status: "online", CustomText: "a\u0007b"}, "customStatus.text"},
		"invalid UTF-8":       {StatusUpdate{Status: "online", CustomText: "a\xffb"}, "customStatus.text"},
		"expiry in the past":  {StatusUpdate{Status: "online", CustomText: "x", ExpiresAt: &past}, "customStatus.expiresAt"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := fixture.service.SetStatus(t.Context(), fixture.user.ID, test.update)

			assert.Equal(t, test.field, fieldOf(t, err))
		})
	}
}

func TestSetStatusErrors(t *testing.T) {
	fixture := newFixture(t)
	_, err := fixture.service.SetStatus(t.Context(), uuid.New(), StatusUpdate{Status: "online"})
	require.ErrorIs(t, err, ErrUserNotFound)
	fixture.users.failOn = "SetStatus"
	_, err = fixture.service.SetStatus(t.Context(), fixture.user.ID, StatusUpdate{Status: "online"})
	assert.ErrorIs(t, err, errFailure)
}

func TestAvatarLifecycle(t *testing.T) {
	fixture := newFixture(t)

	first, err := fixture.service.SetAvatar(t.Context(), fixture.user.ID, testPNG(t), &avatar.Crop{X: 0, Y: 0, Size: 32})
	require.NoError(t, err)
	require.NotNil(t, first.AvatarID)
	picture, err := fixture.service.OpenAvatar(t.Context(), *first.AvatarID)
	require.NoError(t, err)
	assert.Equal(t, "image/png", picture.ContentType)
	content, err := io.ReadAll(picture.Content)
	require.NoError(t, err)
	require.NoError(t, picture.Content.Close())
	assert.NotEmpty(t, content)

	second, err := fixture.service.SetAvatar(t.Context(), fixture.user.ID, testPNG(t), nil)
	require.NoError(t, err)
	assert.NotEqual(t, *first.AvatarID, *second.AvatarID, "a new picture gets a new address")
	_, err = fixture.service.OpenAvatar(t.Context(), *first.AvatarID)
	require.ErrorIs(t, err, ErrAvatarNotFound, "replaced pictures are not served")
	_, err = fixture.files.Local.Open(t.Context(), "avatars/"+first.AvatarID.String())
	require.Error(t, err, "and their file is gone")

	removed, err := fixture.service.RemoveAvatar(t.Context(), fixture.user.ID)
	require.NoError(t, err)
	assert.Nil(t, removed.AvatarID)
	_, err = fixture.service.OpenAvatar(t.Context(), *second.AvatarID)
	require.ErrorIs(t, err, ErrAvatarNotFound)
	_, err = fixture.service.RemoveAvatar(t.Context(), fixture.user.ID)
	assert.NoError(t, err, "removing twice is fine")
}

func TestSetAvatarErrors(t *testing.T) {
	fixture := newFixture(t)
	_, err := fixture.service.SetAvatar(t.Context(), fixture.user.ID, []byte("not an image"), nil)
	require.ErrorIs(t, err, avatar.ErrUnsupported)

	_, err = fixture.service.SetAvatar(t.Context(), uuid.New(), testPNG(t), nil)
	require.ErrorIs(t, err, ErrUserNotFound)

	fixture.files.failOn = "Save"
	_, err = fixture.service.SetAvatar(t.Context(), fixture.user.ID, testPNG(t), nil)
	require.ErrorIs(t, err, errFailure)

	fixture.files.failOn, fixture.users.failOn = "Delete", "SetAvatar"
	_, err = fixture.service.SetAvatar(t.Context(), fixture.user.ID, testPNG(t), nil)
	require.ErrorIs(t, err, errFailure)
	assert.Contains(t, fixture.logs.String(), "could not delete old profile picture")

	_, err = fixture.service.RemoveAvatar(t.Context(), fixture.user.ID)
	require.ErrorIs(t, err, errFailure)
	_, err = fixture.service.RemoveAvatar(t.Context(), uuid.New())
	require.ErrorIs(t, err, ErrUserNotFound)

	fixture.limiter.denied = true
	_, err = fixture.service.SetAvatar(t.Context(), fixture.user.ID, testPNG(t), nil)
	require.ErrorIs(t, err, ErrRateLimited)

	fixture.limiter.denied = false
	canceled, cancel := contextWithCancel(t)
	cancel()
	require.NoError(t, fixture.service.decoding.Acquire(t.Context(), concurrentAvatars))
	_, err = fixture.service.SetAvatar(canceled, fixture.user.ID, testPNG(t), nil)
	assert.ErrorContains(t, err, "waiting to process picture")
}

func TestOpenAvatarErrors(t *testing.T) {
	fixture := newFixture(t)
	user, err := fixture.service.SetAvatar(t.Context(), fixture.user.ID, testPNG(t), nil)
	require.NoError(t, err)

	fixture.files.failOn = "Open"
	_, err = fixture.service.OpenAvatar(t.Context(), *user.AvatarID)
	require.ErrorIs(t, err, errFailure)
	fixture.users.failOn = "GetAvatarFormat"
	_, err = fixture.service.OpenAvatar(t.Context(), *user.AvatarID)
	assert.ErrorIs(t, err, errFailure)
}
