package filestore

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveOpenDelete(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "files")
	store := NewLocal(directory)

	require.NoError(t, store.Save(t.Context(), "avatars/abc-123", []byte("picture")))
	require.NoError(t, store.Save(t.Context(), "avatars/abc-123", []byte("replaced")))
	file, err := store.Open(t.Context(), "avatars/abc-123")
	require.NoError(t, err)
	contents, err := io.ReadAll(file)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	assert.Equal(t, "replaced", string(contents))
	info, err := os.Stat(filepath.Join(directory, "avatars", "abc-123"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	require.NoError(t, store.Delete(t.Context(), "avatars/abc-123"))
	require.NoError(t, store.Delete(t.Context(), "avatars/abc-123"), "deleting twice is fine")
	_, err = store.Open(t.Context(), "avatars/abc-123")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestKeysCannotEscapeTheDirectory(t *testing.T) {
	store := NewLocal(t.TempDir())
	for _, key := range []string{"../secrets", "avatars/../x", "/etc/passwd", `avatars\x`, "flat", "a/b/c", "Avatars/x"} {
		require.ErrorIs(t, store.Save(t.Context(), key, nil), errBadKey, key)
		_, err := store.Open(t.Context(), key)
		require.ErrorIs(t, err, errBadKey, key)
		require.ErrorIs(t, store.Delete(t.Context(), key), errBadKey, key)
	}
}

func TestFailures(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	store := NewLocal(blocker)
	require.ErrorContains(t, store.Save(t.Context(), "avatars/x", nil), "creating file folder")

	directory := t.TempDir()
	store = NewLocal(directory)
	require.NoError(t, os.MkdirAll(filepath.Join(directory, "avatars", "x.tmp"), 0o700))
	require.ErrorContains(t, store.Save(t.Context(), "avatars/x", nil), "writing avatars/x")

	require.NoError(t, os.MkdirAll(filepath.Join(directory, "avatars", "y", "child"), 0o700))
	require.ErrorContains(t, store.Save(t.Context(), "avatars/y", nil), "saving avatars/y")
	require.ErrorContains(t, store.Delete(t.Context(), "avatars/y"), "deleting avatars/y")

	require.NoError(t, os.Chmod(filepath.Join(directory, "avatars"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(directory, "avatars"), 0o700) })
	_, err := store.Open(t.Context(), "avatars/z")
	if os.Geteuid() != 0 { // root ignores permissions
		assert.ErrorContains(t, err, "opening avatars/z")
	}
}

// failingReader fails after giving nothing.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, os.ErrDeadlineExceeded }

func TestSaveStream(t *testing.T) {
	directory := t.TempDir()
	store := NewLocal(directory)

	size, err := store.SaveStream(t.Context(), "attachments/a", strings.NewReader("12345"), 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), size)
	file, err := store.Open(t.Context(), "attachments/a")
	require.NoError(t, err)
	contents, err := io.ReadAll(file)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	assert.Equal(t, "12345", string(contents))

	_, err = store.SaveStream(t.Context(), "attachments/b", strings.NewReader("123456"), 5)
	require.ErrorIs(t, err, ErrTooLarge)
	_, err = store.Open(t.Context(), "attachments/b")
	require.ErrorIs(t, err, ErrNotFound, "nothing is stored")
	_, err = store.SaveStream(t.Context(), "attachments/c", failingReader{}, 5)
	require.ErrorContains(t, err, "writing attachments/c")
	entries, err := os.ReadDir(filepath.Join(directory, "attachments"))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary files are left behind")

	_, err = store.SaveStream(t.Context(), "../x", strings.NewReader(""), 5)
	require.ErrorIs(t, err, errBadKey)
}

func TestSaveStreamFailures(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	_, err := NewLocal(blocker).SaveStream(t.Context(), "attachments/x", strings.NewReader(""), 1)
	require.ErrorContains(t, err, "creating file folder")

	directory := t.TempDir()
	store := NewLocal(directory)
	require.NoError(t, os.MkdirAll(filepath.Join(directory, "attachments", "y", "child"), 0o700))
	_, err = store.SaveStream(t.Context(), "attachments/y", strings.NewReader(""), 1)
	require.ErrorContains(t, err, "saving attachments/y")
	if os.Geteuid() != 0 { // root ignores permissions
		require.NoError(t, os.Chmod(filepath.Join(directory, "attachments"), 0o500))
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(directory, "attachments"), 0o700) })
		_, err = store.SaveStream(t.Context(), "attachments/z", strings.NewReader(""), 1)
		require.ErrorContains(t, err, "writing attachments/z")
	}
}
