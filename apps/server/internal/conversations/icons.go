package conversations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/conversations/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
)

// Group DM icons (spec §4.3), processed like profile pictures: the
// centered square, 512×512, without metadata. Each upload gets a new
// random ID, so icons can be served publicly and cached forever.

// Errors about icons.
var (
	ErrIconNotFound = errors.New("there is no such icon")
	ErrRateLimited  = errors.New("too many icon changes; wait a minute")
)

// Icon is a stored icon ready to serve.
type Icon struct {
	Content     io.ReadCloser
	ContentType string
}

// Icons are what the service needs for group DM icons.
type Icons struct {
	Files   fileStore
	Limiter attemptLimiter
	Logger  *slog.Logger
}

type fileStore interface {
	Save(ctx context.Context, key string, data []byte) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type attemptLimiter interface {
	Allow(key string) bool
}

type iconStore interface {
	SetIcon(ctx context.Context, params store.SetIconParams) error
	GetIconFormat(ctx context.Context, iconID uuid.UUID) (string, error)
}

func iconKey(id uuid.UUID) string {
	return "group-icons/" + id.String()
}

// SetIcon makes upload the icon of a group DM the caller is in.
func (service *Service) SetIcon(ctx context.Context, me uuid.UUID, id uuid.UUID, upload []byte) (Conversation, error) {
	current, err := service.group(ctx, me, id)
	if err != nil {
		return Conversation{}, err
	}
	icons := service.dependencies.Icons
	if !icons.Limiter.Allow(me.String()) {
		return Conversation{}, ErrRateLimited
	}
	processed, err := avatar.Process(upload, nil)
	if err != nil {
		return Conversation{}, err
	}
	iconID := uuid.New()
	if err := icons.Files.Save(ctx, iconKey(iconID), processed.Data); err != nil {
		return Conversation{}, err
	}
	format := string(processed.Format)
	if err := service.dependencies.Store.SetIcon(ctx, store.SetIconParams{ID: id, IconID: &iconID, IconFormat: &format}); err != nil {
		service.deleteIcon(ctx, &iconID)
		return Conversation{}, fmt.Errorf("saving icon: %w", err)
	}
	return service.iconChanged(ctx, me, current)
}

// RemoveIcon removes a group DM's icon.
func (service *Service) RemoveIcon(ctx context.Context, me uuid.UUID, id uuid.UUID) (Conversation, error) {
	current, err := service.group(ctx, me, id)
	if err != nil {
		return Conversation{}, err
	}
	if err := service.dependencies.Store.SetIcon(ctx, store.SetIconParams{ID: id}); err != nil {
		return Conversation{}, fmt.Errorf("removing icon: %w", err)
	}
	return service.iconChanged(ctx, me, current)
}

// iconChanged deletes the previous icon's file, tells the members, and
// returns the conversation as it is now.
func (service *Service) iconChanged(ctx context.Context, me uuid.UUID, previous store.Conversation) (Conversation, error) {
	service.deleteIcon(ctx, previous.IconID)
	if err := service.notifyMembers(ctx, previous.ID); err != nil {
		return Conversation{}, err
	}
	return service.Get(ctx, me, previous.ID)
}

// OpenIcon returns a current icon, or ErrIconNotFound. Replaced icons are
// not served, even while their file still exists.
func (service *Service) OpenIcon(ctx context.Context, iconID uuid.UUID) (Icon, error) {
	format, err := service.dependencies.Store.GetIconFormat(ctx, iconID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Icon{}, ErrIconNotFound
	}
	if err != nil {
		return Icon{}, fmt.Errorf("finding icon: %w", err)
	}
	content, err := service.dependencies.Icons.Files.Open(ctx, iconKey(iconID))
	if err != nil {
		return Icon{}, fmt.Errorf("opening icon: %w", err)
	}
	return Icon{Content: content, ContentType: avatar.Format(format).ContentType()}, nil
}

// deleteIcon removes an icon's file. A failure only leaves an unused file
// behind, which is logged rather than failing the request.
func (service *Service) deleteIcon(ctx context.Context, iconID *uuid.UUID) {
	if iconID == nil {
		return
	}
	if err := service.dependencies.Icons.Files.Delete(ctx, iconKey(*iconID)); err != nil {
		service.dependencies.Icons.Logger.WarnContext(ctx, "could not delete an old group icon", slog.Any("error", err))
	}
}
