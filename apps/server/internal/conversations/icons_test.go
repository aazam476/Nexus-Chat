package conversations

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
)

func pngImage(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 64, 48))))
	return buffer.Bytes()
}

// group makes a group DM of ada, grace and alan, and returns it with its members.
func (f fixture) group(t *testing.T) (Conversation, uuid.UUID, uuid.UUID) {
	t.Helper()
	ada, grace, alan := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "alan")
	group, err := f.service.CreateGroup(t.Context(), ada, nil, []uuid.UUID{grace, alan})
	require.NoError(t, err)
	return group, ada, grace
}

func TestGroupIconsCanBeSetReplacedAndRemoved(t *testing.T) {
	f := newFixture(t)
	group, ada, grace := f.group(t)

	first, err := f.service.SetIcon(t.Context(), ada, group.ID, pngImage(t))
	require.NoError(t, err)
	require.NotNil(t, first.IconID)
	icon, err := f.service.OpenIcon(t.Context(), *first.IconID)
	require.NoError(t, err)
	content, err := io.ReadAll(icon.Content)
	require.NoError(t, err)
	require.NoError(t, icon.Content.Close())
	decoded, err := png.Decode(bytes.NewReader(content))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 512, 512), decoded.Bounds(), "the centered square, resized")
	assert.Equal(t, "image/png", icon.ContentType)

	// Any member may change it; the old icon stops being served.
	second, err := f.service.SetIcon(t.Context(), grace, group.ID, pngImage(t))
	require.NoError(t, err)
	assert.NotEqual(t, first.IconID, second.IconID)
	_, err = f.service.OpenIcon(t.Context(), *first.IconID)
	require.ErrorIs(t, err, ErrIconNotFound)

	removed, err := f.service.RemoveIcon(t.Context(), ada, group.ID)
	require.NoError(t, err)
	assert.Nil(t, removed.IconID)
	_, err = f.service.OpenIcon(t.Context(), *second.IconID)
	require.ErrorIs(t, err, ErrIconNotFound)
	assert.Contains(t, f.events.changed, grace, "members are told")
}

func TestGroupIconsAreForGroupMembersWithImages(t *testing.T) {
	f := newFixture(t)
	group, ada, grace := f.group(t)
	outsider := f.user(t, "mallory")
	dm, _, err := f.service.OpenDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)

	_, err = f.service.SetIcon(t.Context(), ada, dm.ID, pngImage(t))
	require.ErrorIs(t, err, ErrNotAGroup)
	_, err = f.service.RemoveIcon(t.Context(), ada, dm.ID)
	require.ErrorIs(t, err, ErrNotAGroup)
	_, err = f.service.SetIcon(t.Context(), outsider, group.ID, pngImage(t))
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.SetIcon(t.Context(), ada, group.ID, []byte("not an image"))
	require.ErrorIs(t, err, avatar.ErrUnsupported)

	limited := NewService(Dependencies{
		Store: f.service.dependencies.Store, Policy: f.social, Notifier: f.events, Members: f.events, Clock: clock.Fixed{Time: now},
		Icons: Icons{Files: f.service.dependencies.Icons.Files, Limiter: denyAll{}, Logger: slog.New(slog.DiscardHandler)},
	})
	_, err = limited.SetIcon(t.Context(), ada, group.ID, pngImage(t))
	require.ErrorIs(t, err, ErrRateLimited)
}

type denyAll struct{}

func (denyAll) Allow(string) bool { return false }

// brokenFiles fails the file operation named by failOn.
type brokenFiles struct {
	fileStore
	failOn string
}

var errDisk = errors.New("disk full")

func (files brokenFiles) Save(ctx context.Context, key string, data []byte) error {
	if files.failOn == "Save" {
		return errDisk
	}
	return files.fileStore.Save(ctx, key, data)
}

func (files brokenFiles) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if files.failOn == "Open" {
		return nil, errDisk
	}
	return files.fileStore.Open(ctx, key)
}

func (files brokenFiles) Delete(ctx context.Context, key string) error {
	if files.failOn == "Delete" {
		return errDisk
	}
	return files.fileStore.Delete(ctx, key)
}

func TestGroupIconFileFailures(t *testing.T) {
	f := newFixture(t)
	group, ada, _ := f.group(t)
	with := func(failOn string) *Service {
		return NewService(Dependencies{
			Store: f.service.dependencies.Store, Policy: f.social, Notifier: f.events, Members: f.events, Clock: clock.Fixed{Time: now},
			Icons: Icons{Files: brokenFiles{f.service.dependencies.Icons.Files, failOn}, Limiter: allowAll{}, Logger: slog.New(slog.DiscardHandler)},
		})
	}

	_, err := with("Save").SetIcon(t.Context(), ada, group.ID, pngImage(t))
	require.ErrorIs(t, err, errDisk)
	set, err := with("Open").SetIcon(t.Context(), ada, group.ID, pngImage(t))
	require.NoError(t, err)
	_, err = with("Open").OpenIcon(t.Context(), *set.IconID)
	require.ErrorIs(t, err, errDisk)
	// A file that cannot be deleted is only logged.
	_, err = with("Delete").RemoveIcon(t.Context(), ada, group.ID)
	require.NoError(t, err)
	_, err = f.service.OpenIcon(t.Context(), uuid.New())
	require.ErrorIs(t, err, ErrIconNotFound)
}
