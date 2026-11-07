package instanceconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
}

// Service reads and writes instance settings.
type Service struct {
	settings settingsStore
	clock    clock.Clock
}

// NewService returns a Service backed by settings and using clock for the
// current time.
func NewService(settings settingsStore, clock clock.Clock) *Service {
	return &Service{settings: settings, clock: clock}
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
