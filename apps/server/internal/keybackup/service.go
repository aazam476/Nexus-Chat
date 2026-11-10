package keybackup

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/keybackup/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// Limits on what one user stores (docs/e2ee-design.md §5).
const (
	// MaximumBlobs is how many blobs a user may keep: identity keys, a
	// history-key blob per group and month, and search-index chunks.
	MaximumBlobs = 10_000
	// MaximumBlobSize is the largest blob, in bytes.
	MaximumBlobSize = 1024 * 1024
	// maximumWrappedKeySize is far above the 72 bytes a wrapped key takes.
	maximumWrappedKeySize = 256
	maximumParametersSize = 1024
	saltSize              = 16
)

// blobName matches the names the database accepts.
var blobName = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,127}$`) //nolint:gochecknoglobals // compiled once, never changed

// The unlock methods (docs/e2ee-design.md §5).
var methods = map[string]bool{"recovery-phrase": true, "password": true, "passkey": true} //nolint:gochecknoglobals // constant set

// Errors returned to the caller.
var (
	ErrUnknownMethod   = errors.New("the unlock method must be recovery-phrase, password or passkey")
	ErrInvalidWrapped  = fmt.Errorf("a wrapped key is 1 to %d bytes, with at most %d bytes of parameters", maximumWrappedKeySize, maximumParametersSize)
	ErrInvalidBlobName = errors.New("a blob name is lowercase letters, digits and _.-, up to 128, starting with a letter or digit")
	ErrInvalidBlob     = fmt.Errorf("a blob holds 1 byte to %d bytes", MaximumBlobSize)
	ErrTooManyBlobs    = fmt.Errorf("a backup may hold at most %d blobs", MaximumBlobs)
	ErrNotFound        = errors.New("there is no such item in the backup")
)

// WrappedKey is the backup key wrapped under one unlock method.
type WrappedKey struct {
	Method     string
	WrappedKey []byte
	Parameters []byte
}

// BlobInfo describes a stored blob without its data.
type BlobInfo struct {
	Name      string
	Size      int
	UpdatedAt time.Time
}

// Backup is everything a device needs to start restoring.
type Backup struct {
	Salt        []byte
	WrappedKeys []WrappedKey
	Blobs       []BlobInfo
}

type backupStore interface {
	EnsureBackup(ctx context.Context, params store.EnsureBackupParams) ([]byte, error)
	ListWrappedKeys(ctx context.Context, userID uuidType) ([]store.ListWrappedKeysRow, error)
	PutWrappedKey(ctx context.Context, params store.PutWrappedKeyParams) error
	DeleteWrappedKey(ctx context.Context, params store.DeleteWrappedKeyParams) (int64, error)
	ListBlobs(ctx context.Context, userID uuidType) ([]store.ListBlobsRow, error)
	CountBlobs(ctx context.Context, userID uuidType) (int64, error)
	GetBlob(ctx context.Context, params store.GetBlobParams) ([]byte, error)
	PutBlob(ctx context.Context, params store.PutBlobParams) error
	DeleteBlob(ctx context.Context, params store.DeleteBlobParams) (int64, error)
}

// Service is the key-backup store.
type Service struct {
	store backupStore
	clock clock.Clock
}

// NewService returns a Service.
func NewService(backups backupStore, clock clock.Clock) *Service {
	return &Service{store: backups, clock: clock}
}

// Get returns the caller's salt (created on first use), wrapped keys and
// the list of blobs.
func (service *Service) Get(ctx context.Context, caller session.Principal) (Backup, error) {
	salt := make([]byte, saltSize)
	// crypto/rand.Read never returns an error; it stops the program if the
	// system's random source fails.
	_, _ = rand.Read(salt)
	stored, err := service.store.EnsureBackup(ctx, store.EnsureBackupParams{UserID: caller.UserID, Salt: salt, CreatedAt: service.clock.Now()})
	if err != nil {
		return Backup{}, fmt.Errorf("creating the backup salt: %w", err)
	}
	wrappedRows, err := service.store.ListWrappedKeys(ctx, caller.UserID)
	if err != nil {
		return Backup{}, fmt.Errorf("listing wrapped keys: %w", err)
	}
	blobRows, err := service.store.ListBlobs(ctx, caller.UserID)
	if err != nil {
		return Backup{}, fmt.Errorf("listing blobs: %w", err)
	}
	backup := Backup{Salt: stored, WrappedKeys: make([]WrappedKey, 0, len(wrappedRows)), Blobs: make([]BlobInfo, 0, len(blobRows))}
	for _, row := range wrappedRows {
		backup.WrappedKeys = append(backup.WrappedKeys, WrappedKey(row))
	}
	for _, row := range blobRows {
		backup.Blobs = append(backup.Blobs, BlobInfo{Name: row.Name, Size: int(row.Size), UpdatedAt: row.UpdatedAt})
	}
	return backup, nil
}

// PutWrappedKey adds or replaces the caller's copy for one unlock method.
func (service *Service) PutWrappedKey(ctx context.Context, caller session.Principal, key WrappedKey) error {
	if !methods[key.Method] {
		return ErrUnknownMethod
	}
	if len(key.WrappedKey) == 0 || len(key.WrappedKey) > maximumWrappedKeySize || len(key.Parameters) > maximumParametersSize {
		return ErrInvalidWrapped
	}
	if err := caller.RequireRecent(service.clock.Now()); err != nil {
		return err
	}
	return service.store.PutWrappedKey(ctx, store.PutWrappedKeyParams{
		UserID: caller.UserID, Method: key.Method, WrappedKey: key.WrappedKey, Parameters: key.Parameters, UpdatedAt: service.clock.Now(),
	})
}

// DeleteWrappedKey turns off one unlock method for the caller.
func (service *Service) DeleteWrappedKey(ctx context.Context, caller session.Principal, method string) error {
	if !methods[method] {
		return ErrUnknownMethod
	}
	if err := caller.RequireRecent(service.clock.Now()); err != nil {
		return err
	}
	deleted, err := service.store.DeleteWrappedKey(ctx, store.DeleteWrappedKeyParams{UserID: caller.UserID, Method: method})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}
