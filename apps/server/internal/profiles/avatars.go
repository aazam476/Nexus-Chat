package profiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
)

// Picture is a stored profile picture ready to serve.
type Picture struct {
	Content     io.ReadCloser
	ContentType string
}

func avatarKey(id uuid.UUID) string {
	return "avatars/" + id.String()
}

// SetAvatar processes an upload and makes it the user's picture. The old
// picture's file is deleted afterwards.
func (service *Service) SetAvatar(ctx context.Context, userID uuid.UUID, upload []byte, crop *avatar.Crop) (store.User, error) {
	if !service.dependencies.AvatarLimiter.Allow(userID.String()) {
		return store.User{}, ErrRateLimited
	}
	if err := service.decoding.Acquire(ctx, 1); err != nil {
		return store.User{}, fmt.Errorf("waiting to process picture: %w", err)
	}
	processed, err := avatar.Process(upload, crop)
	service.decoding.Release(1)
	if err != nil {
		return store.User{}, err
	}
	current, err := service.Get(ctx, userID)
	if err != nil {
		return store.User{}, err
	}
	id := uuid.Must(uuid.NewV7())
	if err := service.dependencies.Files.Save(ctx, avatarKey(id), processed.Data); err != nil {
		return store.User{}, err
	}
	format := string(processed.Format)
	user, err := service.dependencies.Store.SetAvatar(ctx, store.SetAvatarParams{ID: userID, AvatarID: &id, AvatarFormat: &format})
	if err != nil {
		service.deleteFile(ctx, id)
		return store.User{}, fmt.Errorf("saving picture: %w", err)
	}
	service.deleteOld(ctx, current)
	return service.changed(ctx, user, nil)
}

// RemoveAvatar removes the user's picture.
func (service *Service) RemoveAvatar(ctx context.Context, userID uuid.UUID) (store.User, error) {
	current, err := service.Get(ctx, userID)
	if err != nil {
		return store.User{}, err
	}
	user, err := service.dependencies.Store.SetAvatar(ctx, store.SetAvatarParams{ID: userID})
	if err != nil {
		return store.User{}, fmt.Errorf("removing picture: %w", err)
	}
	service.deleteOld(ctx, current)
	return service.changed(ctx, user, nil)
}

// OpenAvatar returns a current profile picture, or ErrAvatarNotFound.
// Replaced pictures are not served, even while their file still exists.
func (service *Service) OpenAvatar(ctx context.Context, id uuid.UUID) (Picture, error) {
	format, err := service.dependencies.Store.GetAvatarFormat(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Picture{}, ErrAvatarNotFound
	}
	if err != nil {
		return Picture{}, fmt.Errorf("finding picture: %w", err)
	}
	content, err := service.dependencies.Files.Open(ctx, avatarKey(id))
	if err != nil {
		return Picture{}, fmt.Errorf("opening picture: %w", err)
	}
	return Picture{Content: content, ContentType: avatar.Format(format).ContentType()}, nil
}

func (service *Service) deleteOld(ctx context.Context, previous store.User) {
	if previous.AvatarID != nil {
		service.deleteFile(ctx, *previous.AvatarID)
	}
}

// deleteFile removes a picture's file. A failure only leaves an unused
// file behind, which is logged rather than failing the request.
func (service *Service) deleteFile(ctx context.Context, id uuid.UUID) {
	if err := service.dependencies.Files.Delete(ctx, avatarKey(id)); err != nil {
		service.dependencies.Logger.WarnContext(ctx, "could not delete old profile picture", slog.Any("error", err))
	}
}
