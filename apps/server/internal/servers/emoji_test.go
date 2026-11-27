package servers

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/filestore"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

func bigPNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 256, 128))))
	return buffer.Bytes()
}

func twoFrameGIF(t *testing.T) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	frames := []*image.Paletted{image.NewPaletted(image.Rect(0, 0, 8, 8), palette), image.NewPaletted(image.Rect(0, 0, 8, 8), palette)}
	var buffer bytes.Buffer
	require.NoError(t, gif.EncodeAll(&buffer, &gif.GIF{Image: frames, Delay: []int{5, 5}}))
	return buffer.Bytes()
}

func encodeJPEG(writer io.Writer) error {
	return jpeg.Encode(writer, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil)
}

func readPicture(t *testing.T, picture Icon) image.Image {
	t.Helper()
	content, err := io.ReadAll(picture.Content)
	require.NoError(t, err)
	require.NoError(t, picture.Content.Close())
	decoded, _, err := image.Decode(bytes.NewReader(content))
	require.NoError(t, err)
	return decoded
}

func TestCustomEmojiAreAddedListedServedAndRemoved(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	owner, member := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}

	wave, err := f.service.AddEmoji(t.Context(), owner, "wave", bigPNG(t))
	require.NoError(t, err)
	party, err := f.service.AddEmoji(t.Context(), owner, "party_parrot", twoFrameGIF(t))
	require.NoError(t, err)
	assert.Contains(t, f.events.changed, grace, "members are told")

	list, err := f.service.ListEmoji(t.Context(), member)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, []string{"party_parrot", "wave"}, []string{list[0].Name, list[1].Name}, "by name")
	assert.True(t, list[0].Animated)
	assert.False(t, list[1].Animated)
	assert.Equal(t, &ada, list[1].CreatedBy)

	picture, err := f.service.OpenEmoji(t.Context(), wave)
	require.NoError(t, err)
	assert.Equal(t, "image/png", picture.ContentType)
	assert.Equal(t, image.Rect(0, 0, 128, 64), readPicture(t, picture).Bounds(), "scaled down, keeping its shape")
	animated, err := f.service.OpenEmoji(t.Context(), party)
	require.NoError(t, err)
	assert.Equal(t, "image/gif", animated.ContentType)
	require.NoError(t, animated.Content.Close())

	require.NoError(t, f.service.RemoveEmoji(t.Context(), owner, wave))
	_, err = f.service.OpenEmoji(t.Context(), wave)
	require.ErrorIs(t, err, ErrEmojiNotFound)
	require.ErrorIs(t, f.service.RemoveEmoji(t.Context(), owner, wave), ErrEmojiNotFound)
}

func TestWebPEmojiAreStoredAsPNG(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	owner := Scope{f.server(t, ada).Server.ID, ada}
	data, err := os.ReadFile("../profiles/avatar/testdata/still.webp")
	require.NoError(t, err)

	id, err := f.service.AddEmoji(t.Context(), owner, "still", data)

	require.NoError(t, err)
	picture, err := f.service.OpenEmoji(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "image/png", picture.ContentType)
	require.NoError(t, picture.Content.Close())
}

func TestEmojiThatAreRefused(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	owner, member := Scope{details.Server.ID, ada}, Scope{details.Server.ID, grace}
	_, err := f.service.AddEmoji(t.Context(), owner, "wave", pngImage(t))
	require.NoError(t, err)
	var jpeg bytes.Buffer
	require.NoError(t, encodeJPEG(&jpeg))

	tests := []struct {
		name  string
		who   Scope
		emoji string
		data  []byte
		want  error
	}{
		{"without MANAGE_EMOJI", member, "hello", pngImage(t), ErrForbidden},
		{"by a stranger", Scope{details.Server.ID, f.user(t, "eve")}, "hello", pngImage(t), ErrNotFound},
		{"name too short", owner, "a", pngImage(t), ErrEmojiName},
		{"name with capitals", owner, "Wave", pngImage(t), ErrEmojiName},
		{"name too long", owner, "abcdefghijklmnopqrstuvwxyz0123456", pngImage(t), ErrEmojiName},
		{"name taken", owner, "wave", pngImage(t), ErrEmojiTaken},
		{"JPEG", owner, "photo", jpeg.Bytes(), avatar.ErrNotAllowed},
		{"not a picture", owner, "text", []byte("hello"), avatar.ErrUnsupported},
		{"over 256 KB", owner, "huge", make([]byte, EmojiMaximumBytes+1), avatar.ErrTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.service.AddEmoji(t.Context(), test.who, test.emoji, test.data)

			assert.ErrorIs(t, err, test.want)
		})
	}
	require.ErrorIs(t, f.service.RemoveEmoji(t.Context(), member, uuid.New()), ErrForbidden)
	require.ErrorIs(t, f.service.RemoveEmoji(t.Context(), Scope{details.Server.ID, f.user(t, "sam")}, uuid.New()), ErrNotFound)
	_, err = f.service.ListEmoji(t.Context(), Scope{details.Server.ID, f.user(t, "zed")})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestAServerHasAtMostAHundredEmoji(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	owner := Scope{f.server(t, ada).Server.ID, ada}
	for index := range emojiPerServer {
		require.NoError(t, f.store.InsertServerEmoji(t.Context(), store.InsertServerEmojiParams{
			ID: uuid.New(), ServerID: owner.ServerID, Name: "e" + string(rune('a'+index/26)) + string(rune('a'+index%26)), Format: "png", CreatedAt: start,
		}))
	}

	_, err := f.service.AddEmoji(t.Context(), owner, "one_more", pngImage(t))

	require.ErrorIs(t, err, ErrEmojiLimit)
	list, err := f.service.ListEmoji(t.Context(), owner)
	require.NoError(t, err)
	assert.Len(t, list, emojiPerServer)
}

func TestDeletingAServerRemovesItsPictures(t *testing.T) {
	f := newFixture(t)
	directory := t.TempDir()
	files := filestore.NewLocal(directory)
	service := NewService(Dependencies{Store: f.store, Notifier: f.events, Audiences: f.leaves, Icons: Icons{Files: files, Limiter: allowAll{}, Logger: slog.New(slog.DiscardHandler)}, Clock: f.clock})
	ada := f.user(t, "ada")
	details, err := service.Create(t.Context(), ada, "Book club", "")
	require.NoError(t, err)
	owner := Scope{details.Server.ID, ada}
	require.NoError(t, service.SetIcon(t.Context(), owner, pngImage(t)))
	id, err := service.AddEmoji(t.Context(), owner, "wave", pngImage(t))
	require.NoError(t, err)
	withIcon, err := service.Details(t.Context(), owner)
	require.NoError(t, err)

	require.NoError(t, service.Delete(t.Context(), owner))

	for _, key := range []string{emojiKey(id), iconKey(*withIcon.Server.IconID)} {
		_, err := files.Open(t.Context(), key)
		assert.Error(t, err, key)
	}
}

func TestEmojiFailures(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	owner := Scope{f.server(t, ada).Server.ID, ada}
	files := filestore.NewLocal(t.TempDir())
	quiet := slog.New(slog.DiscardHandler)
	with := func(icons Icons) *Service {
		return NewService(Dependencies{Store: f.store, Notifier: f.events, Audiences: f.leaves, Icons: icons, Clock: f.clock})
	}

	_, err := with(Icons{Files: files, Limiter: denyAll{}, Logger: quiet}).AddEmoji(t.Context(), owner, "wave", pngImage(t))
	require.ErrorIs(t, err, ErrRateLimited)
	_, err = with(Icons{Files: brokenFiles{files, "save"}, Limiter: allowAll{}, Logger: quiet}).AddEmoji(t.Context(), owner, "wave", pngImage(t))
	require.Error(t, err)

	// A file that cannot be deleted only leaves an unused file behind.
	deleting := with(Icons{Files: brokenFiles{files, "delete"}, Limiter: allowAll{}, Logger: quiet})
	id, err := deleting.AddEmoji(t.Context(), owner, "wave", pngImage(t))
	require.NoError(t, err)
	_, err = with(Icons{Files: brokenFiles{files, "open"}, Limiter: allowAll{}, Logger: quiet}).OpenEmoji(t.Context(), id)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrEmojiNotFound)
	require.NoError(t, deleting.RemoveEmoji(t.Context(), owner, id))

	f.pool.Close()
	_, err = f.service.AddEmoji(t.Context(), owner, "late", pngImage(t))
	require.Error(t, err)
	_, err = f.service.ListEmoji(t.Context(), owner)
	require.Error(t, err)
	require.Error(t, f.service.RemoveEmoji(t.Context(), owner, id))
	_, err = f.service.OpenEmoji(t.Context(), id)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrEmojiNotFound)
}

func TestEmojiEndpoints(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	id := f.server(t, ada, grace).Server.ID
	handler := NewHandler(f.service)
	add := func(who uuid.UUID, name string, body []byte) string {
		response, err := handler.AddServerEmoji(as(t, who), api.AddServerEmojiRequestObject{ServerId: id, Params: api.AddServerEmojiParams{Name: name}, Body: bytes.NewReader(body)})
		require.NoError(t, err)
		return typeName(response)
	}

	assert.Equal(t, "AddServerEmoji201JSONResponse", add(ada, "wave", pngImage(t)))
	assert.Equal(t, "AddServerEmoji403JSONResponse", add(grace, "hello", pngImage(t)))
	assert.Equal(t, "AddServerEmoji404JSONResponse", add(f.user(t, "eve"), "hello", pngImage(t)))
	assert.Equal(t, "AddServerEmoji409JSONResponse", add(ada, "wave", pngImage(t)))
	assert.Equal(t, "AddServerEmoji413JSONResponse", add(ada, "huge", make([]byte, EmojiMaximumBytes+2)))
	assert.Equal(t, "AddServerEmoji422JSONResponse", add(ada, "No", pngImage(t)))
	assert.Equal(t, "AddServerEmoji422JSONResponse", add(ada, "text", []byte("hello")))

	listed := must(handler.ListServerEmoji(as(t, grace), api.ListServerEmojiRequestObject{ServerId: id}))
	list, ok := listed.(api.ListServerEmoji200JSONResponse)
	require.True(t, ok)
	require.Len(t, list, 1)
	assert.Equal(t, "/api/v1/emoji/"+list[0].Id.String(), list[0].Url)
	served := must(handler.GetEmoji(t.Context(), api.GetEmojiRequestObject{EmojiId: list[0].Id}))
	picture, ok := served.(api.GetEmoji200ImagepngResponse)
	require.True(t, ok)
	assert.Equal(t, iconCacheControl, picture.Headers.CacheControl)
	require.NoError(t, picture.Body.(io.Closer).Close())

	remove := func(who uuid.UUID, emoji uuid.UUID) string {
		return typeName(must(handler.RemoveServerEmoji(as(t, who), api.RemoveServerEmojiRequestObject{ServerId: id, EmojiId: emoji})))
	}
	assert.Equal(t, "RemoveServerEmoji403JSONResponse", remove(grace, list[0].Id))
	assert.Equal(t, "RemoveServerEmoji404JSONResponse", remove(f.user(t, "sam"), list[0].Id))
	assert.Equal(t, "RemoveServerEmoji204Response", remove(ada, list[0].Id))
	assert.Equal(t, "RemoveServerEmoji404JSONResponse", remove(ada, list[0].Id))
	assert.Equal(t, "GetEmoji404JSONResponse", typeName(must(handler.GetEmoji(t.Context(), api.GetEmojiRequestObject{EmojiId: list[0].Id}))))
	assert.Equal(t, "ListServerEmoji404JSONResponse", typeName(must(handler.ListServerEmoji(as(t, f.user(t, "zed")), api.ListServerEmojiRequestObject{ServerId: id}))))
}

func TestAnimatedEmojiAreServedAsGIF(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	owner := Scope{f.server(t, ada).Server.ID, ada}
	emoji, err := f.service.AddEmoji(t.Context(), owner, "party", twoFrameGIF(t))
	require.NoError(t, err)

	served := must(NewHandler(f.service).GetEmoji(t.Context(), api.GetEmojiRequestObject{EmojiId: emoji}))

	picture, ok := served.(api.GetEmoji200ImagegifResponse)
	require.True(t, ok)
	require.NoError(t, picture.Body.(io.Closer).Close())
}

func TestEmojiLimitsInTheStore(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	serverID := f.server(t, ada).Server.ID
	row := func(name string) store.InsertServerEmojiParams {
		return store.InsertServerEmojiParams{ID: uuid.New(), ServerID: serverID, Name: name, Format: "png", CreatedAt: start}
	}

	require.NoError(t, f.store.AddEmoji(t.Context(), row("one"), 1))
	require.ErrorIs(t, f.store.AddEmoji(t.Context(), row("two"), 1), store.ErrEmojiLimit)
	require.ErrorIs(t, f.store.AddEmoji(t.Context(), row("one"), 5), store.ErrEmojiNameTaken)
	require.Error(t, f.store.AddEmoji(t.Context(), store.InsertServerEmojiParams{ID: uuid.New(), ServerID: uuid.New(), Name: "x1", Format: "png", CreatedAt: start}, 5), "no such server")
}
