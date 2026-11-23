package servers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/filestore"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
)

func pngImage(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 64, 48))))
	return buffer.Bytes()
}

func TestServerIconsCanBeSetReplacedAndRemoved(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	owner, member := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}

	require.NoError(t, f.service.SetIcon(t.Context(), owner, pngImage(t)))
	first, err := f.service.Details(t.Context(), member)
	require.NoError(t, err)
	require.NotNil(t, first.Server.IconID)
	icon, err := f.service.OpenIcon(t.Context(), *first.Server.IconID)
	require.NoError(t, err)
	content, err := io.ReadAll(icon.Content)
	require.NoError(t, err)
	require.NoError(t, icon.Content.Close())
	decoded, err := png.Decode(bytes.NewReader(content))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 512, 512), decoded.Bounds(), "the centered square, resized")
	assert.Equal(t, "image/png", icon.ContentType)

	// A new icon replaces the old one, which stops being served.
	require.NoError(t, f.service.SetIcon(t.Context(), owner, pngImage(t)))
	_, err = f.service.OpenIcon(t.Context(), *first.Server.IconID)
	require.ErrorIs(t, err, ErrIconNotFound)
	require.ErrorIs(t, f.service.SetIcon(t.Context(), member, pngImage(t)), ErrForbidden)
	require.ErrorIs(t, f.service.SetIcon(t.Context(), owner, []byte("not an image")), avatar.ErrUnsupported)
	require.ErrorIs(t, f.service.RemoveIcon(t.Context(), member), ErrForbidden)

	require.NoError(t, f.service.RemoveIcon(t.Context(), owner))
	removed, err := f.service.Details(t.Context(), owner)
	require.NoError(t, err)
	assert.Nil(t, removed.Server.IconID)
	assert.Contains(t, f.events.changed, grace, "members are told")
}

type denyAll struct{}

func (denyAll) Allow(string) bool { return false }

// brokenFiles fails the operation named failOn.
type brokenFiles struct {
	FileStore
	failOn string
}

func (files brokenFiles) Save(ctx context.Context, key string, data []byte) error {
	if files.failOn == "save" {
		return errors.New("disk full")
	}
	return files.FileStore.Save(ctx, key, data)
}

func (files brokenFiles) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if files.failOn == "open" {
		return nil, errors.New("disk gone")
	}
	return files.FileStore.Open(ctx, key)
}

func (files brokenFiles) Delete(ctx context.Context, key string) error {
	if files.failOn == "delete" {
		return errors.New("read-only disk")
	}
	return files.FileStore.Delete(ctx, key)
}

func TestServerIconFailures(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	details := f.server(t, ada)
	owner := Scope{details.Server.ID, ada}
	with := func(icons Icons) *Service {
		return NewService(Dependencies{Store: f.store, Notifier: f.events, Audiences: f.leaves, Icons: icons, Clock: f.clock})
	}
	files := filestore.NewLocal(t.TempDir())
	quiet := slog.New(slog.DiscardHandler)

	require.ErrorIs(t, with(Icons{Files: files, Limiter: denyAll{}, Logger: quiet}).SetIcon(t.Context(), owner, pngImage(t)), ErrRateLimited)
	require.Error(t, with(Icons{Files: brokenFiles{files, "save"}, Limiter: allowAll{}, Logger: quiet}).SetIcon(t.Context(), owner, pngImage(t)))

	// A file that cannot be deleted only leaves an unused file behind.
	deleting := with(Icons{Files: brokenFiles{files, "delete"}, Limiter: allowAll{}, Logger: quiet})
	require.NoError(t, deleting.SetIcon(t.Context(), owner, pngImage(t)))
	require.NoError(t, deleting.SetIcon(t.Context(), owner, pngImage(t)))
	current, err := deleting.Details(t.Context(), owner)
	require.NoError(t, err)
	_, err = with(Icons{Files: brokenFiles{files, "open"}, Limiter: allowAll{}, Logger: quiet}).OpenIcon(t.Context(), *current.Server.IconID)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrIconNotFound)
	require.ErrorIs(t, deleting.SetIcon(t.Context(), Scope{details.Server.ID, f.user(t, "eve")}, pngImage(t)), ErrNotFound)
	require.ErrorIs(t, deleting.RemoveIcon(t.Context(), Scope{details.Server.ID, f.user(t, "sam")}), ErrNotFound)

	f.pool.Close()
	_, err = f.service.OpenIcon(t.Context(), *current.Server.IconID)
	require.Error(t, err)
	require.Error(t, f.service.SetIcon(t.Context(), owner, pngImage(t)))
	require.Error(t, f.service.RemoveIcon(t.Context(), owner))
}

func TestServerIconEndpoints(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	handler := NewHandler(f.service)
	id := details.Server.ID
	put := func(who Scope, body []byte) api.SetServerIconResponseObject {
		response, err := handler.SetServerIcon(as(t, who.UserID), api.SetServerIconRequestObject{ServerId: id, Body: bytes.NewReader(body)})
		require.NoError(t, err)
		return response
	}

	assert.IsType(t, api.SetServerIcon204Response{}, put(Scope{id, ada}, pngImage(t)))
	assert.IsType(t, api.SetServerIcon403JSONResponse{}, put(Scope{id, grace}, pngImage(t)))
	assert.IsType(t, api.SetServerIcon422JSONResponse{}, put(Scope{id, ada}, []byte("nope")))
	assert.IsType(t, api.SetServerIcon413JSONResponse{}, put(Scope{id, ada}, make([]byte, 9<<20)))
	assert.IsType(t, api.SetServerIcon404JSONResponse{}, put(Scope{id, f.user(t, "eve")}, pngImage(t)))
	got, err := handler.GetServer(as(t, grace), api.GetServerRequestObject{ServerId: id})
	require.NoError(t, err)
	server := got.(api.GetServer200JSONResponse).Server //nolint:forcetypeassert // the success type
	require.NotNil(t, server.IconUrl)
	current, err := f.service.Details(t.Context(), Scope{id, ada})
	require.NoError(t, err)
	served := must(handler.GetServerIcon(t.Context(), api.GetServerIconRequestObject{IconId: *current.Server.IconID}))
	png, ok := served.(api.GetServerIcon200ImagepngResponse)
	require.True(t, ok)
	assert.Equal(t, iconCacheControl, png.Headers.CacheControl)
	require.NoError(t, png.Body.(io.Closer).Close())

	for _, response := range []any{
		must(handler.RemoveServerIcon(as(t, grace), api.RemoveServerIconRequestObject{ServerId: id})),
		must(handler.RemoveServerIcon(as(t, f.user(t, "sam")), api.RemoveServerIconRequestObject{ServerId: id})),
		must(handler.RemoveServerIcon(as(t, ada), api.RemoveServerIconRequestObject{ServerId: id})),
		must(handler.GetServerIcon(t.Context(), api.GetServerIconRequestObject{IconId: uuid.New()})),
	} {
		assert.Contains(t, []string{"RemoveServerIcon403JSONResponse", "RemoveServerIcon404JSONResponse", "RemoveServerIcon204Response", "GetServerIcon404JSONResponse"}, typeName(response))
	}
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func typeName(value any) string {
	name := fmt.Sprintf("%T", value)
	return name[strings.LastIndex(name, ".")+1:]
}
