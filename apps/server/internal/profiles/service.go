package profiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/semaphore"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// Field limits (spec §4.1).
const (
	maximumBio          = 500
	maximumCustomStatus = 128
	// concurrentAvatars bounds how many uploads are decoded at once: each
	// may need up to 256 MiB.
	concurrentAvatars = 2
)

// Errors for the API.
var (
	ErrUserNotFound   = errors.New("there is no such user")
	ErrAvatarNotFound = errors.New("there is no such picture")
	ErrRateLimited    = errors.New("too many picture uploads; wait a minute")
)

type profileStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (store.User, error)
	UpdateProfile(ctx context.Context, arg store.UpdateProfileParams) (store.User, error)
	SetAvatar(ctx context.Context, arg store.SetAvatarParams) (store.User, error)
	GetAvatarFormat(ctx context.Context, avatarID uuid.UUID) (string, error)
	SetStatus(ctx context.Context, arg store.SetStatusParams) (store.User, error)
}

type fileStore interface {
	Save(ctx context.Context, key string, data []byte) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// ChangeListener hears about every change to an account, so the gateway can
// tell the user's devices and update presence.
type ChangeListener interface {
	UserChanged(ctx context.Context, user store.User)
}

type attemptLimiter interface {
	Allow(key string) bool
}

// Dependencies are what Service needs.
type Dependencies struct {
	Store    profileStore
	Files    fileStore
	Listener ChangeListener
	// AvatarLimiter bounds picture uploads per user.
	AvatarLimiter attemptLimiter
	Clock         clock.Clock
	Logger        *slog.Logger
}

// Service changes and reads profiles.
type Service struct {
	dependencies Dependencies
	decoding     *semaphore.Weighted
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies, decoding: semaphore.NewWeighted(concurrentAvatars)}
}

// Get returns the account userID, or ErrUserNotFound.
func (service *Service) Get(ctx context.Context, userID uuid.UUID) (store.User, error) {
	user, err := service.dependencies.Store.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, ErrUserNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("loading profile: %w", err)
	}
	return user, nil
}

// ProfileUpdate changes the fields that are not nil.
type ProfileUpdate struct {
	DisplayName *string
	Bio         *string
}

// UpdateProfile validates and saves update.
func (service *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, update ProfileUpdate) (store.User, error) {
	current, err := service.Get(ctx, userID)
	if err != nil {
		return store.User{}, err
	}
	displayName, bio := current.DisplayName, current.Bio
	if update.DisplayName != nil {
		displayName = *update.DisplayName
	}
	if update.Bio != nil {
		bio = strings.TrimSpace(*update.Bio)
	}
	var problems validation.Errors
	problems.Add("displayName", accounts.ValidateDisplayName(displayName))
	if utf8.RuneCountInString(bio) > maximumBio || !utf8.ValidString(bio) {
		problems.Add("bio", fmt.Errorf("must be at most %d characters", maximumBio))
	}
	if err := problems.Err(); err != nil {
		return store.User{}, err
	}
	user, err := service.dependencies.Store.UpdateProfile(ctx, store.UpdateProfileParams{ID: userID, DisplayName: displayName, Bio: bio})
	return service.changed(ctx, user, err)
}

// StatusUpdate is the status the user chose and their custom status. An
// empty CustomText removes the custom status.
type StatusUpdate struct {
	Status     string
	CustomText string
	ExpiresAt  *time.Time
}

// SetStatus validates and saves update.
func (service *Service) SetStatus(ctx context.Context, userID uuid.UUID, update StatusUpdate) (store.User, error) {
	params, err := service.checkStatus(userID, update)
	if err != nil {
		return store.User{}, err
	}
	user, err := service.dependencies.Store.SetStatus(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, ErrUserNotFound
	}
	return service.changed(ctx, user, err)
}

func (service *Service) checkStatus(userID uuid.UUID, update StatusUpdate) (store.SetStatusParams, error) {
	var problems validation.Errors
	switch update.Status {
	case accounts.StatusOnline, accounts.StatusIdle, accounts.StatusDND, accounts.StatusInvisible:
	default:
		problems.Add("status", errors.New("must be online, idle, dnd or invisible"))
	}
	params := store.SetStatusParams{ID: userID, Status: update.Status}
	text := strings.Join(strings.Fields(update.CustomText), " ")
	switch {
	case text == "" && update.ExpiresAt != nil:
		problems.Add("customStatus.text", errors.New("is required when an expiry is set"))
	case utf8.RuneCountInString(text) > maximumCustomStatus || strings.ContainsFunc(text, unicode.IsControl) || !utf8.ValidString(text):
		problems.Add("customStatus.text", fmt.Errorf("must be 1–%d characters", maximumCustomStatus))
	case update.ExpiresAt != nil && !update.ExpiresAt.After(service.dependencies.Clock.Now()):
		problems.Add("customStatus.expiresAt", errors.New("must be in the future"))
	case text != "":
		params.CustomStatusText, params.CustomStatusExpiresAt = &text, update.ExpiresAt
	}
	return params, problems.Err()
}

// changed passes on the result of a write and tells the listener.
func (service *Service) changed(ctx context.Context, user store.User, err error) (store.User, error) {
	if err != nil {
		return store.User{}, fmt.Errorf("saving profile: %w", err)
	}
	service.dependencies.Listener.UserChanged(ctx, user)
	return user, nil
}
