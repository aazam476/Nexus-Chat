package servers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// Custom emoji (spec §4.5): PNG, GIF or WebP up to 256 KB, kept at up to
// 128×128 and encoded again without metadata (WebP becomes PNG), named
// [a-z0-9_]{2,32}, at most 100 per server. Like server icons they are
// public assets under a random ID: messages that use them are encrypted,
// but the pictures are not.

// Limits on custom emoji.
const (
	EmojiMaximumBytes = 256 << 10
	emojiMaximumSide  = 128
	emojiPerServer    = 100
)

// Errors about emoji.
var (
	ErrEmojiName     = errors.New("an emoji name is 2 to 32 characters: lowercase letters, digits and underscores")
	ErrEmojiNotFound = errors.New("there is no such emoji")
	ErrEmojiLimit    = fmt.Errorf("a server can have at most %d emoji", emojiPerServer)
	ErrEmojiTaken    = errors.New("this server already has an emoji with that name")
)

var emojiName = regexp.MustCompile(`^[a-z0-9_]{2,32}$`)

// emojiLimits are what an emoji picture may be.
func emojiLimits() avatar.Limits {
	return avatar.Limits{MaximumBytes: EmojiMaximumBytes, MaximumSide: emojiMaximumSide, Formats: []string{"png", "gif", "webp"}}
}

// Emoji is one of a server's custom emoji.
type Emoji struct {
	ID        uuid.UUID
	Name      string
	Animated  bool
	CreatedBy *uuid.UUID
	CreatedAt time.Time
}

func emojiKey(id uuid.UUID) string {
	return "server-emoji/" + id.String()
}

// ListEmoji returns the server's emoji by name, to any member.
func (service *Service) ListEmoji(ctx context.Context, scope Scope) ([]Emoji, error) {
	if _, err := service.load(ctx, scope); err != nil {
		return nil, err
	}
	rows, err := service.store.ServerEmoji(ctx, scope.ServerID)
	if err != nil {
		return nil, fmt.Errorf("listing emoji: %w", err)
	}
	list := make([]Emoji, len(rows))
	for index, row := range rows {
		list[index] = Emoji{ID: row.ID, Name: row.Name, Animated: row.Format == string(avatar.FormatGIF), CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt}
	}
	return list, nil
}

// AddEmoji adds upload as the emoji name (MANAGE_EMOJI) and returns its ID.
func (service *Service) AddEmoji(ctx context.Context, scope Scope, name string, upload []byte) (uuid.UUID, error) {
	a, err := service.load(ctx, scope)
	if err != nil {
		return uuid.Nil, err
	}
	if err := a.require(permissions.ManageEmoji); err != nil {
		return uuid.Nil, err
	}
	if !emojiName.MatchString(name) {
		return uuid.Nil, ErrEmojiName
	}
	if !service.icons.Limiter.Allow(scope.UserID.String()) {
		return uuid.Nil, ErrRateLimited
	}
	processed, err := avatar.Fit(upload, emojiLimits())
	if err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	if err := service.icons.Files.Save(ctx, emojiKey(id), processed.Data); err != nil {
		return uuid.Nil, err
	}
	creator := scope.UserID
	row := store.InsertServerEmojiParams{ID: id, ServerID: scope.ServerID, Name: name, Format: string(processed.Format), CreatedBy: &creator, CreatedAt: service.clock.Now()}
	if err := service.store.AddEmoji(ctx, row, emojiPerServer); err != nil {
		service.deleteFile(ctx, emojiKey(id))
		return uuid.Nil, emojiStoreError(err)
	}
	return id, service.announce(ctx, scope.ServerID)
}

func emojiStoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrEmojiLimit):
		return ErrEmojiLimit
	case errors.Is(err, store.ErrEmojiNameTaken):
		return ErrEmojiTaken
	}
	return fmt.Errorf("saving emoji: %w", err)
}

// RemoveEmoji deletes one of the server's emoji (MANAGE_EMOJI).
func (service *Service) RemoveEmoji(ctx context.Context, scope Scope, emojiID uuid.UUID) error {
	a, err := service.load(ctx, scope)
	if err != nil {
		return err
	}
	if err := a.require(permissions.ManageEmoji); err != nil {
		return err
	}
	_, err = service.store.DeleteServerEmoji(ctx, store.DeleteServerEmojiParams{ID: emojiID, ServerID: scope.ServerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEmojiNotFound
	}
	if err != nil {
		return fmt.Errorf("deleting emoji: %w", err)
	}
	service.deleteFile(ctx, emojiKey(emojiID))
	return service.announce(ctx, scope.ServerID)
}

// OpenEmoji returns an existing emoji's picture, or ErrEmojiNotFound.
func (service *Service) OpenEmoji(ctx context.Context, emojiID uuid.UUID) (Icon, error) {
	format, err := service.store.GetServerEmojiFormat(ctx, emojiID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Icon{}, ErrEmojiNotFound
	}
	if err != nil {
		return Icon{}, fmt.Errorf("finding emoji: %w", err)
	}
	content, err := service.icons.Files.Open(ctx, emojiKey(emojiID))
	if err != nil {
		return Icon{}, fmt.Errorf("opening emoji: %w", err)
	}
	return Icon{Content: content, ContentType: avatar.Format(format).ContentType()}, nil
}

// deleteFile removes a picture's file. A failure only leaves an unused
// file behind, which is logged rather than failing the request.
func (service *Service) deleteFile(ctx context.Context, key string) {
	if err := service.icons.Files.Delete(ctx, key); err != nil {
		service.icons.Logger.WarnContext(ctx, "could not delete an unused picture", slog.String("key", key), slog.Any("error", err))
	}
}
