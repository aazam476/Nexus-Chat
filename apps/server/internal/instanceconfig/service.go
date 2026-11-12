package instanceconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// firstBootKey is the settings key that records when the instance first
// started.
const firstBootKey = "instance.first_boot_at"

// settingsStore is the subset of the generated store the service needs.
// Defining it here (where it is used) lets tests pass a small fake.
type settingsStore interface {
	InsertSettingIfMissing(ctx context.Context, arg store.InsertSettingIfMissingParams) (store.InstanceSetting, error)
	GetSetting(ctx context.Context, key string) (store.InstanceSetting, error)
	UpsertSetting(ctx context.Context, arg store.UpsertSettingParams) error
}

// ErrNoDomain means the instance has no public domain yet, so it cannot
// build links for emails.
var ErrNoDomain = errors.New("the instance has no public domain; set DOMAIN or set it in the Admin panel")

// Service reads and writes instance settings.
type Service struct {
	settings settingsStore
	clock    clock.Clock
	// environmentDomain is DOMAIN from .env, which overrides the stored one.
	environmentDomain string
}

// NewService returns a Service backed by settings and using clock for the
// current time. environmentDomain is DOMAIN from .env (may be empty).
func NewService(settings settingsStore, clock clock.Clock, environmentDomain string) *Service {
	return &Service{settings: settings, clock: clock, environmentDomain: environmentDomain}
}

// Load decodes the JSON setting key into target. found is false if the
// setting has never been set.
func (service *Service) Load(ctx context.Context, key string, target any) (bool, error) {
	setting, err := service.settings.GetSetting(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading setting %s: %w", key, err)
	}
	if err := json.Unmarshal(setting.Value, target); err != nil {
		return false, fmt.Errorf("decoding setting %s: %w", key, err)
	}
	return true, nil
}

// Save stores value as the JSON setting key, replacing any earlier value.
func (service *Service) Save(ctx context.Context, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding setting %s: %w", key, err)
	}
	if err := service.settings.UpsertSetting(ctx, store.UpsertSettingParams{Key: key, Value: encoded}); err != nil {
		return fmt.Errorf("saving setting %s: %w", key, err)
	}
	return nil
}

// Domain returns the public domain: DOMAIN from .env if set, otherwise the
// one saved in setup or the Admin panel, otherwise "".
func (service *Service) Domain(ctx context.Context) (string, error) {
	if service.environmentDomain != "" {
		return service.environmentDomain, nil
	}
	var domain string
	_, err := service.Load(ctx, KeyInstanceDomain, &domain)
	return domain, err
}

// BaseURL returns the instance's public HTTPS address, for links in
// emails, or ErrNoDomain.
func (service *Service) BaseURL(ctx context.Context) (string, error) {
	domain, err := service.Domain(ctx)
	if err != nil {
		return "", err
	}
	if domain == "" {
		return "", ErrNoDomain
	}
	return "https://" + domain, nil
}

// RegistrationMode returns "open", "invite_only" or "closed". An instance
// that never saved a mode is invite-only, the safe default.
func (service *Service) RegistrationMode(ctx context.Context) (string, error) {
	mode := "invite_only"
	_, err := service.Load(ctx, KeyRegistrationMode, &mode)
	return mode, err
}

// RecordFirstBoot stores the current time as the first boot time if none is
// stored yet, and returns the stored first boot time either way.
func (service *Service) RecordFirstBoot(ctx context.Context) (time.Time, error) {
	value, err := json.Marshal(service.clock.Now())
	if err != nil {
		return time.Time{}, fmt.Errorf("encoding first boot time: %w", err)
	}
	setting, err := service.settings.InsertSettingIfMissing(ctx, store.InsertSettingIfMissingParams{
		Key:   firstBootKey,
		Value: value,
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("recording first boot time: %w", err)
	}
	var firstBoot time.Time
	if err := json.Unmarshal(setting.Value, &firstBoot); err != nil {
		return time.Time{}, fmt.Errorf("decoding stored first boot time: %w", err)
	}
	return firstBoot, nil
}

// InstanceName returns the name set in setup, or "Nexus Chat".
func (service *Service) InstanceName(ctx context.Context) (string, error) {
	name := "Nexus Chat"
	_, err := service.Load(ctx, KeyInstanceName, &name)
	return name, err
}

// Attachment limits (ADR 0014). The limit counts plaintext bytes; the
// encrypted file is a little larger, which the attachments feature allows.
const (
	DefaultAttachmentMaxBytes int64 = 25 << 20
	MinimumAttachmentMaxBytes int64 = 1 << 20
	MaximumAttachmentMaxBytes int64 = 4 << 30
)

// ErrInvalidAttachmentLimit means a limit outside 1 MiB to 4 GiB.
var ErrInvalidAttachmentLimit = errors.New("the attachment limit must be between 1 MiB and 4 GiB")

// AttachmentMaxBytes returns the instance's attachment limit, 25 MiB unless
// the admin changed it.
func (service *Service) AttachmentMaxBytes(ctx context.Context) (int64, error) {
	limit := DefaultAttachmentMaxBytes
	_, err := service.Load(ctx, KeyAttachmentMaxBytes, &limit)
	return limit, err
}

// SetAttachmentMaxBytes changes the instance's attachment limit.
func (service *Service) SetAttachmentMaxBytes(ctx context.Context, limit int64) error {
	if limit < MinimumAttachmentMaxBytes || limit > MaximumAttachmentMaxBytes {
		return ErrInvalidAttachmentLimit
	}
	return service.Save(ctx, KeyAttachmentMaxBytes, limit)
}

// LinkPreviewProxy reports whether the admin turned on the link preview
// proxy, which lets the web app show previews; it is off by default because
// the instance then sees the links people preview.
func (service *Service) LinkPreviewProxy(ctx context.Context) (bool, error) {
	enabled := false
	_, err := service.Load(ctx, KeyLinkPreviewProxy, &enabled)
	return enabled, err
}

// SetLinkPreviewProxy turns the link preview proxy on or off.
func (service *Service) SetLinkPreviewProxy(ctx context.Context, enabled bool) error {
	return service.Save(ctx, KeyLinkPreviewProxy, enabled)
}
