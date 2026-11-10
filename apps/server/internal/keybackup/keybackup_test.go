package keybackup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/keybackup/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

var errDown = errors.New("database down")

var now = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

// fakeStore keeps one user's backup in memory; failing makes the named call fail.
type fakeStore struct {
	salt    []byte
	wrapped map[string]store.ListWrappedKeysRow
	blobs   map[string][]byte
	count   int64 // overrides len(blobs) when set
	failing string
}

func newFake() *fakeStore {
	return &fakeStore{wrapped: map[string]store.ListWrappedKeysRow{}, blobs: map[string][]byte{}}
}

func (fake *fakeStore) fail(call string) error {
	if fake.failing == call {
		return errDown
	}
	return nil
}

func (fake *fakeStore) EnsureBackup(_ context.Context, params store.EnsureBackupParams) ([]byte, error) {
	if fake.salt == nil {
		fake.salt = params.Salt
	}
	return fake.salt, fake.fail("EnsureBackup")
}

func (fake *fakeStore) ListWrappedKeys(context.Context, uuid.UUID) ([]store.ListWrappedKeysRow, error) {
	rows := []store.ListWrappedKeysRow{}
	for _, method := range []string{"passkey", "password", "recovery-phrase"} {
		if row, ok := fake.wrapped[method]; ok {
			rows = append(rows, row)
		}
	}
	return rows, fake.fail("ListWrappedKeys")
}

func (fake *fakeStore) PutWrappedKey(_ context.Context, params store.PutWrappedKeyParams) error {
	fake.wrapped[params.Method] = store.ListWrappedKeysRow{Method: params.Method, WrappedKey: params.WrappedKey, Parameters: params.Parameters}
	return fake.fail("PutWrappedKey")
}

func (fake *fakeStore) DeleteWrappedKey(_ context.Context, params store.DeleteWrappedKeyParams) (int64, error) {
	_, ok := fake.wrapped[params.Method]
	delete(fake.wrapped, params.Method)
	if ok {
		return 1, fake.fail("DeleteWrappedKey")
	}
	return 0, fake.fail("DeleteWrappedKey")
}

func (fake *fakeStore) ListBlobs(context.Context, uuid.UUID) ([]store.ListBlobsRow, error) {
	rows := []store.ListBlobsRow{}
	for name, data := range fake.blobs {
		rows = append(rows, store.ListBlobsRow{Name: name, Size: int32(len(data)), UpdatedAt: now}) //nolint:gosec // small test data
	}
	return rows, fake.fail("ListBlobs")
}

func (fake *fakeStore) CountBlobs(context.Context, uuid.UUID) (int64, error) {
	if fake.count > 0 {
		return fake.count, fake.fail("CountBlobs")
	}
	return int64(len(fake.blobs)), fake.fail("CountBlobs")
}

func (fake *fakeStore) GetBlob(_ context.Context, params store.GetBlobParams) ([]byte, error) {
	if err := fake.fail("GetBlob"); err != nil {
		return nil, err
	}
	data, ok := fake.blobs[params.Name]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return data, nil
}

func (fake *fakeStore) PutBlob(_ context.Context, params store.PutBlobParams) error {
	fake.blobs[params.Name] = params.Data
	return fake.fail("PutBlob")
}

func (fake *fakeStore) DeleteBlob(_ context.Context, params store.DeleteBlobParams) (int64, error) {
	_, ok := fake.blobs[params.Name]
	delete(fake.blobs, params.Name)
	if ok {
		return 1, fake.fail("DeleteBlob")
	}
	return 0, fake.fail("DeleteBlob")
}

func setUp() (*Handler, *fakeStore) {
	fake := newFake()
	return NewHandler(NewService(fake, clock.Fixed{Time: now})), fake
}

// signedIn is a caller whose last login was `ago` before now.
func signedIn(t *testing.T, ago time.Duration) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New(), SessionID: uuid.New(), AuthenticatedAt: now.Add(-ago)})
}

func putWrapped(method string, wrapped []byte, parameters *[]byte) api.PutWrappedBackupKeyRequestObject {
	return api.PutWrappedBackupKeyRequestObject{Method: api.UnlockMethod(method), Body: &api.PutWrappedBackupKeyJSONRequestBody{WrappedKey: wrapped, Parameters: parameters}}
}

func TestTheSaltIsMadeOnceAndTheBackupListed(t *testing.T) {
	handler, fake := setUp()
	ctx := signedIn(t, time.Minute)
	parameters := []byte("argon2id")
	_, err := handler.PutWrappedBackupKey(ctx, putWrapped("password", []byte{1, 2}, &parameters))
	require.NoError(t, err)
	_, err = handler.PutWrappedBackupKey(ctx, putWrapped("recovery-phrase", []byte{3}, nil))
	require.NoError(t, err)
	_, err = handler.PutBackupBlob(ctx, api.PutBackupBlobRequestObject{Name: "identity", Body: &api.PutBackupBlobJSONRequestBody{Data: []byte{9, 9}}})
	require.NoError(t, err)

	first, err := handler.GetKeyBackup(ctx, api.GetKeyBackupRequestObject{})
	require.NoError(t, err)
	second, err := handler.GetKeyBackup(ctx, api.GetKeyBackupRequestObject{})
	require.NoError(t, err)

	backup := first.(api.GetKeyBackup200JSONResponse) //nolint:forcetypeassert // the only success type
	assert.Len(t, backup.Salt, 16)
	assert.Equal(t, fake.salt, backup.Salt)
	assert.Equal(t, first, second, "the salt never changes")
	assert.Equal(t, []api.WrappedBackupKey{
		{Method: "password", WrappedKey: []byte{1, 2}, Parameters: &parameters},
		{Method: "recovery-phrase", WrappedKey: []byte{3}},
	}, backup.WrappedKeys)
	assert.Equal(t, []api.BackupBlobInfo{{Name: "identity", Size: 2, UpdatedAt: now}}, backup.Blobs)
}

func TestUnlockMethodsNeedARecentLoginAndValidInput(t *testing.T) {
	handler, fake := setUp()
	stale := signedIn(t, time.Hour)

	refused, err := handler.PutWrappedBackupKey(stale, putWrapped("passkey", []byte{1}, nil))
	require.NoError(t, err)
	assert.IsType(t, api.PutWrappedBackupKey403JSONResponse{}, refused)
	assert.Empty(t, fake.wrapped)

	fresh := signedIn(t, time.Minute)
	unknown, err := handler.PutWrappedBackupKey(fresh, putWrapped("email", []byte{1}, nil))
	require.NoError(t, err)
	assert.IsType(t, api.PutWrappedBackupKey400JSONResponse{}, unknown)
	tooLong := make([]byte, 1025)
	for _, request := range []api.PutWrappedBackupKeyRequestObject{
		putWrapped("passkey", nil, nil), putWrapped("passkey", make([]byte, 257), nil), putWrapped("passkey", []byte{1}, &tooLong),
	} {
		invalid, err := handler.PutWrappedBackupKey(fresh, request)
		require.NoError(t, err)
		assert.IsType(t, api.PutWrappedBackupKey422JSONResponse{}, invalid)
	}

	stored, err := handler.PutWrappedBackupKey(fresh, putWrapped("passkey", []byte{1}, nil))
	require.NoError(t, err)
	assert.Equal(t, api.PutWrappedBackupKey204Response{}, stored)
	// Exactly at the limits is allowed.
	largest := make([]byte, 1024)
	atLimit, err := handler.PutWrappedBackupKey(fresh, putWrapped("password", make([]byte, 256), &largest))
	require.NoError(t, err)
	assert.Equal(t, api.PutWrappedBackupKey204Response{}, atLimit)
}

func TestTurningOffAnUnlockMethod(t *testing.T) {
	handler, fake := setUp()
	fake.wrapped["passkey"] = store.ListWrappedKeysRow{Method: "passkey", WrappedKey: []byte{1}}
	fresh := signedIn(t, time.Minute)
	request := func(method string) api.DeleteWrappedBackupKeyRequestObject {
		return api.DeleteWrappedBackupKeyRequestObject{Method: api.UnlockMethod(method)}
	}

	stale, err := handler.DeleteWrappedBackupKey(signedIn(t, time.Hour), request("passkey"))
	require.NoError(t, err)
	assert.IsType(t, api.DeleteWrappedBackupKey403JSONResponse{}, stale)
	unknown, err := handler.DeleteWrappedBackupKey(fresh, request("email"))
	require.NoError(t, err)
	assert.IsType(t, api.DeleteWrappedBackupKey400JSONResponse{}, unknown)

	deleted, err := handler.DeleteWrappedBackupKey(fresh, request("passkey"))
	require.NoError(t, err)
	assert.Equal(t, api.DeleteWrappedBackupKey204Response{}, deleted)
	again, err := handler.DeleteWrappedBackupKey(fresh, request("passkey"))
	require.NoError(t, err)
	assert.IsType(t, api.DeleteWrappedBackupKey404JSONResponse{}, again)
}

func TestBlobs(t *testing.T) {
	handler, fake := setUp()
	ctx := signedIn(t, time.Hour) // blobs need no recent login
	put := func(name string, data []byte) api.PutBackupBlobResponseObject {
		response, err := handler.PutBackupBlob(ctx, api.PutBackupBlobRequestObject{Name: name, Body: &api.PutBackupBlobJSONRequestBody{Data: data}})
		require.NoError(t, err)
		return response
	}

	assert.Equal(t, api.PutBackupBlob204Response{}, put("history.g1.2026-11", []byte{1}))
	read, err := handler.GetBackupBlob(ctx, api.GetBackupBlobRequestObject{Name: "history.g1.2026-11"})
	require.NoError(t, err)
	assert.Equal(t, api.GetBackupBlob200JSONResponse{Data: []byte{1}}, read)
	for _, name := range []string{"", "Identity", "../x", "-x", string(make([]byte, 129))} {
		assert.IsType(t, api.PutBackupBlob400JSONResponse{}, put(name, []byte{1}), name)
	}
	assert.IsType(t, api.PutBackupBlob422JSONResponse{}, put("empty", nil))
	assert.IsType(t, api.PutBackupBlob422JSONResponse{}, put("huge", make([]byte, MaximumBlobSize+1)))
	assert.Equal(t, api.PutBackupBlob204Response{}, put("largest", make([]byte, MaximumBlobSize)))
	delete(fake.blobs, "largest")

	// A full backup refuses new blobs but still replaces existing ones.
	fake.count = MaximumBlobs
	assert.IsType(t, api.PutBackupBlob409JSONResponse{}, put("new", []byte{1}))
	assert.Equal(t, api.PutBackupBlob204Response{}, put("history.g1.2026-11", []byte{2}))
	fake.count = 0

	deleted, err := handler.DeleteBackupBlob(ctx, api.DeleteBackupBlobRequestObject{Name: "history.g1.2026-11"})
	require.NoError(t, err)
	assert.Equal(t, api.DeleteBackupBlob204Response{}, deleted)
	gone, err := handler.DeleteBackupBlob(ctx, api.DeleteBackupBlobRequestObject{Name: "history.g1.2026-11"})
	require.NoError(t, err)
	assert.IsType(t, api.DeleteBackupBlob404JSONResponse{}, gone)
	missing, err := handler.GetBackupBlob(ctx, api.GetBackupBlobRequestObject{Name: "history.g1.2026-11"})
	require.NoError(t, err)
	assert.IsType(t, api.GetBackupBlob404JSONResponse{}, missing)
}

func TestStoreFailuresAreReported(t *testing.T) {
	ctx := signedIn(t, time.Minute)
	calls := map[string]func(*Handler) error{
		"EnsureBackup":    func(h *Handler) error { _, err := h.GetKeyBackup(ctx, api.GetKeyBackupRequestObject{}); return err },
		"ListWrappedKeys": func(h *Handler) error { _, err := h.GetKeyBackup(ctx, api.GetKeyBackupRequestObject{}); return err },
		"ListBlobs":       func(h *Handler) error { _, err := h.GetKeyBackup(ctx, api.GetKeyBackupRequestObject{}); return err },
		"PutWrappedKey": func(h *Handler) error {
			_, err := h.PutWrappedBackupKey(ctx, putWrapped("passkey", []byte{1}, nil))
			return err
		},
		"DeleteWrappedKey": func(h *Handler) error {
			_, err := h.DeleteWrappedBackupKey(ctx, api.DeleteWrappedBackupKeyRequestObject{Method: "passkey"})
			return err
		},
		"CountBlobs": func(h *Handler) error {
			_, err := h.PutBackupBlob(ctx, api.PutBackupBlobRequestObject{Name: "a", Body: &api.PutBackupBlobJSONRequestBody{Data: []byte{1}}})
			return err
		},
		"PutBlob": func(h *Handler) error {
			_, err := h.PutBackupBlob(ctx, api.PutBackupBlobRequestObject{Name: "a", Body: &api.PutBackupBlobJSONRequestBody{Data: []byte{1}}})
			return err
		},
		"GetBlob": func(h *Handler) error {
			_, err := h.GetBackupBlob(ctx, api.GetBackupBlobRequestObject{Name: "a"})
			return err
		},
		"DeleteBlob": func(h *Handler) error {
			_, err := h.DeleteBackupBlob(ctx, api.DeleteBackupBlobRequestObject{Name: "a"})
			return err
		},
	}
	for call, run := range calls {
		t.Run(call, func(t *testing.T) {
			handler, fake := setUp()
			fake.failing = call
			require.ErrorIs(t, run(handler), errDown)
		})
	}

	t.Run("checking a blob in a full backup", func(t *testing.T) {
		handler, fake := setUp()
		fake.count = MaximumBlobs
		fake.failing = "GetBlob"
		_, err := handler.PutBackupBlob(ctx, api.PutBackupBlobRequestObject{Name: "a", Body: &api.PutBackupBlobJSONRequestBody{Data: []byte{1}}})
		require.ErrorIs(t, err, errDown)
	})
}

func TestHandlersNeedASession(t *testing.T) {
	handler, _ := setUp()
	ctx := t.Context()
	_, err := handler.GetKeyBackup(ctx, api.GetKeyBackupRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.PutWrappedBackupKey(ctx, putWrapped("passkey", []byte{1}, nil))
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.DeleteWrappedBackupKey(ctx, api.DeleteWrappedBackupKeyRequestObject{Method: "passkey"})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.GetBackupBlob(ctx, api.GetBackupBlobRequestObject{Name: "a"})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.PutBackupBlob(ctx, api.PutBackupBlobRequestObject{Name: "a", Body: &api.PutBackupBlobJSONRequestBody{Data: []byte{1}}})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.DeleteBackupBlob(ctx, api.DeleteBackupBlobRequestObject{Name: "a"})
	require.ErrorIs(t, err, errNoPrincipal)
}
