// Package filestore keeps uploaded files: avatars now, and encrypted
// attachments, emoji and icons later. The default store is a folder in the
// data volume (spec §2); an S3-compatible store can implement the same
// methods.
//
// Start reading at [Local].
package filestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// ErrNotFound means no file is stored under the key.
var ErrNotFound = errors.New("file not found")

// keyPattern allows "folder/name" keys only, so a key can never escape
// the store's directory ("..", absolute paths, backslashes).
var keyPattern = regexp.MustCompile(`^[a-z0-9-]+/[a-z0-9-]+$`)

var errBadKey = errors.New("file keys look like folder/name, using a–z, 0–9 and dashes")

// Local stores files in a directory.
type Local struct {
	directory string
}

// NewLocal returns a store in directory, which is created when the first
// file is saved.
func NewLocal(directory string) *Local {
	return &Local{directory: directory}
}

// Save stores data under key, replacing any earlier file. The file is
// written to a temporary name first, so a reader never sees half of it.
func (store *Local) Save(_ context.Context, key string, data []byte) error {
	path, err := store.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating file folder: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", key, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("saving %s: %w", key, err)
	}
	return nil
}

// Open returns the file stored under key, or ErrNotFound. The caller
// closes it.
func (store *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := store.path(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path) //nolint:gosec // G304: path is checked by store.path
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", key, err)
	}
	return file, nil
}

// Delete removes the file under key. Deleting a missing file is not an
// error, so a retried cleanup always succeeds.
func (store *Local) Delete(_ context.Context, key string) error {
	path, err := store.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("deleting %s: %w", key, err)
	}
	return nil
}

func (store *Local) path(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", errBadKey
	}
	return filepath.Join(store.directory, filepath.FromSlash(key)), nil
}
