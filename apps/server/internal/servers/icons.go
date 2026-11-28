package servers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// Server icons (spec §4.4), processed like profile pictures: the centered
// square, 512×512, without metadata. Each upload gets a new random ID, so
// icons can be served publicly and cached forever.

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

// Icons are what the service needs for server icons.
type Icons struct {
	Files   FileStore
	Limiter AttemptLimiter
	Logger  *slog.Logger
}

// FileStore keeps icon files.
type FileStore interface {
	Save(ctx context.Context, key string, data []byte) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// AttemptLimiter limits how often one user changes icons.
type AttemptLimiter interface {
	Allow(key string) bool
}

func iconKey(id uuid.UUID) string {
	return "server-icons/" + id.String()
}

// SetIcon makes upload the server's icon (MANAGE_SERVER).
func (service *Service) SetIcon(ctx context.Context, scope Scope, upload []byte) error {
	a, err := service.load(ctx, scope)
	if err != nil {
		return err
	}
	if err := a.require(permissions.ManageServer); err != nil {
		return err
	}
	if !service.icons.Limiter.Allow(scope.UserID.String()) {
		return ErrRateLimited
	}
	processed, err := avatar.Process(upload, nil)
	if err != nil {
		return err
	}
	iconID := uuid.New()
	if err := service.icons.Files.Save(ctx, iconKey(iconID), processed.Data); err != nil {
		return err
	}
	format := string(processed.Format)
	if err := service.store.SetServerIcon(ctx, store.SetServerIconParams{ID: scope.ServerID, IconID: &iconID, IconFormat: &format}); err != nil {
		service.deleteIcon(ctx, &iconID)
		return fmt.Errorf("saving icon: %w", err)
	}
	service.deleteIcon(ctx, a.server.IconID)
	changed := event(ActionServerIcon, TargetKindServer, scope.ServerID, map[string]any{"icon": "set"})
	return service.logged(ctx, scope, changed, service.notify(ctx, scope.ServerID))
}

// RemoveIcon removes the server's icon (MANAGE_SERVER).
func (service *Service) RemoveIcon(ctx context.Context, scope Scope) error {
	a, err := service.load(ctx, scope)
	if err != nil {
		return err
	}
	if err := a.require(permissions.ManageServer); err != nil {
		return err
	}
	if err := service.store.SetServerIcon(ctx, store.SetServerIconParams{ID: scope.ServerID}); err != nil {
		return fmt.Errorf("removing icon: %w", err)
	}
	service.deleteIcon(ctx, a.server.IconID)
	changed := event(ActionServerIcon, TargetKindServer, scope.ServerID, map[string]any{"icon": "removed"})
	return service.logged(ctx, scope, changed, service.notify(ctx, scope.ServerID))
}

// OpenIcon returns a current icon, or ErrIconNotFound. Replaced icons are
// not served, even while their file still exists.
func (service *Service) OpenIcon(ctx context.Context, iconID uuid.UUID) (Icon, error) {
	format, err := service.store.GetServerIconFormat(ctx, iconID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Icon{}, ErrIconNotFound
	}
	if err != nil {
		return Icon{}, fmt.Errorf("finding icon: %w", err)
	}
	content, err := service.icons.Files.Open(ctx, iconKey(iconID))
	if err != nil {
		return Icon{}, fmt.Errorf("opening icon: %w", err)
	}
	return Icon{Content: content, ContentType: avatar.Format(format).ContentType()}, nil
}

// deleteIcon removes an icon's file, if there is one.
func (service *Service) deleteIcon(ctx context.Context, iconID *uuid.UUID) {
	if iconID != nil {
		service.deleteFile(ctx, iconKey(*iconID))
	}
}
