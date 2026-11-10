package keybackup

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/keybackup/store"
)

// uuidType keeps the store interface's lines short.
type uuidType = uuid.UUID

// PutBlob adds or replaces one of the caller's blobs. Adding past
// MaximumBlobs is refused; replacing is always allowed. Two uploads at the
// same moment can pass the count together, so the limit may be exceeded
// by a few, which is harmless.
func (service *Service) PutBlob(ctx context.Context, caller session.Principal, name string, data []byte) error {
	if !blobName.MatchString(name) {
		return ErrInvalidBlobName
	}
	if len(data) == 0 || len(data) > MaximumBlobSize {
		return ErrInvalidBlob
	}
	count, err := service.store.CountBlobs(ctx, caller.UserID)
	if err != nil {
		return err
	}
	if count >= MaximumBlobs {
		if _, err := service.GetBlob(ctx, caller, name); err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrTooManyBlobs
			}
			return err
		}
	}
	return service.store.PutBlob(ctx, store.PutBlobParams{UserID: caller.UserID, Name: name, Data: data, UpdatedAt: service.clock.Now()})
}

// GetBlob returns one of the caller's blobs.
func (service *Service) GetBlob(ctx context.Context, caller session.Principal, name string) ([]byte, error) {
	data, err := service.store.GetBlob(ctx, store.GetBlobParams{UserID: caller.UserID, Name: name})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return data, err
}

// DeleteBlob removes one of the caller's blobs.
func (service *Service) DeleteBlob(ctx context.Context, caller session.Principal, name string) error {
	deleted, err := service.store.DeleteBlob(ctx, store.DeleteBlobParams{UserID: caller.UserID, Name: name})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}
