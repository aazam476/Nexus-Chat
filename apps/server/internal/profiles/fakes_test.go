package profiles

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/filestore"
	"github.com/aazam476/nexus-chat/apps/server/internal/realtime"
)

var (
	errFailure = errors.New("database down")
	testNow    = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
)

// fakeUsers keeps users in memory. failOn makes a method fail.
type fakeUsers struct {
	users  map[uuid.UUID]store.User
	failOn string
}

func (fake *fakeUsers) GetUserByID(_ context.Context, id uuid.UUID) (store.User, error) {
	if fake.failOn == "GetUserByID" {
		return store.User{}, errFailure
	}
	user, found := fake.users[id]
	if !found {
		return store.User{}, pgx.ErrNoRows
	}
	return user, nil
}

func (fake *fakeUsers) update(id uuid.UUID, method string, change func(*store.User)) (store.User, error) {
	if fake.failOn == method {
		return store.User{}, errFailure
	}
	user, found := fake.users[id]
	if !found {
		return store.User{}, pgx.ErrNoRows
	}
	change(&user)
	fake.users[id] = user
	return user, nil
}

func (fake *fakeUsers) UpdateProfile(_ context.Context, arg store.UpdateProfileParams) (store.User, error) {
	return fake.update(arg.ID, "UpdateProfile", func(user *store.User) { user.DisplayName, user.Bio = arg.DisplayName, arg.Bio })
}

func (fake *fakeUsers) SetAvatar(_ context.Context, arg store.SetAvatarParams) (store.User, error) {
	return fake.update(arg.ID, "SetAvatar", func(user *store.User) { user.AvatarID, user.AvatarFormat = arg.AvatarID, arg.AvatarFormat })
}

func (fake *fakeUsers) SetStatus(_ context.Context, arg store.SetStatusParams) (store.User, error) {
	return fake.update(arg.ID, "SetStatus", func(user *store.User) {
		user.Status, user.CustomStatusText, user.CustomStatusExpiresAt = arg.Status, arg.CustomStatusText, arg.CustomStatusExpiresAt
	})
}

func (fake *fakeUsers) GetAvatarFormat(_ context.Context, avatarID uuid.UUID) (string, error) {
	if fake.failOn == "GetAvatarFormat" {
		return "", errFailure
	}
	for _, user := range fake.users {
		if user.AvatarID != nil && *user.AvatarID == avatarID {
			return *user.AvatarFormat, nil
		}
	}
	return "", pgx.ErrNoRows
}

// fakeFiles wraps a real local store so failures can be injected.
type fakeFiles struct {
	*filestore.Local
	failOn string
}

func (fake *fakeFiles) Save(ctx context.Context, key string, data []byte) error {
	if fake.failOn == "Save" {
		return errFailure
	}
	return fake.Local.Save(ctx, key, data)
}

func (fake *fakeFiles) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if fake.failOn == "Open" {
		return nil, errFailure
	}
	return fake.Local.Open(ctx, key)
}

func (fake *fakeFiles) Delete(ctx context.Context, key string) error {
	if fake.failOn == "Delete" {
		return errFailure
	}
	return fake.Local.Delete(ctx, key)
}

type fakeListener struct {
	mutex   sync.Mutex
	changes []store.User
}

func (fake *fakeListener) UserChanged(_ context.Context, user store.User) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.changes = append(fake.changes, user)
}

type fakeLimiter struct{ denied bool }

func (fake *fakeLimiter) Allow(string) bool { return !fake.denied }

type fakePresence struct{}

func (fakePresence) PresenceOf(user store.User) realtime.Presence {
	return realtime.Presence{UserID: user.ID, Status: "idle"}
}

type fixture struct {
	service  *Service
	handler  *Handler
	users    *fakeUsers
	files    *fakeFiles
	listener *fakeListener
	limiter  *fakeLimiter
	user     store.User
	logs     *bytes.Buffer
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	user := store.User{ID: uuid.New(), Username: "ada", DisplayName: "Ada", Email: "ada@example.com", Status: "online"}
	logs := &bytes.Buffer{}
	result := fixture{
		users: &fakeUsers{users: map[uuid.UUID]store.User{user.ID: user}}, files: &fakeFiles{Local: filestore.NewLocal(t.TempDir())},
		listener: &fakeListener{}, limiter: &fakeLimiter{}, user: user, logs: logs,
	}
	result.service = NewService(Dependencies{
		Store: result.users, Files: result.files, Listener: result.listener, AvatarLimiter: result.limiter,
		Clock: clock.Fixed{Time: testNow}, Logger: slog.New(slog.NewTextHandler(logs, nil)),
	})
	result.handler = NewHandler(result.service, fakePresence{})
	return result
}

// testPNG is a 64×64 opaque image.
func testPNG(t *testing.T) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for index := range picture.Pix {
		picture.Pix[index] = 0xFF
	}
	picture.Set(0, 0, color.Black)
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, picture))
	return buffer.Bytes()
}

func contextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithCancel(t.Context())
}
