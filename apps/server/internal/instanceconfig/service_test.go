package instanceconfig

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// fakeSettingsStore keeps settings in a map and behaves like the real
// InsertSettingIfMissing query: an existing value always wins.
type fakeSettingsStore struct {
	values map[string][]byte
	err    error
}

func (fake *fakeSettingsStore) InsertSettingIfMissing(_ context.Context, arg store.InsertSettingIfMissingParams) (store.InstanceSetting, error) {
	if fake.err != nil {
		return store.InstanceSetting{}, fake.err
	}
	if _, exists := fake.values[arg.Key]; !exists {
		fake.values[arg.Key] = arg.Value
	}
	return store.InstanceSetting{Key: arg.Key, Value: fake.values[arg.Key]}, nil
}

var firstInstant = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func TestRecordFirstBootStoresTheCurrentTimeOnce(t *testing.T) {
	settings := &fakeSettingsStore{values: map[string][]byte{}}

	first, err := NewService(settings, clock.Fixed{Time: firstInstant}).RecordFirstBoot(context.Background())
	require.NoError(t, err)
	later, err := NewService(settings, clock.Fixed{Time: firstInstant.Add(time.Hour)}).RecordFirstBoot(context.Background())
	require.NoError(t, err)

	assert.Equal(t, firstInstant, first)
	assert.Equal(t, firstInstant, later, "a later boot must not overwrite the first boot time")
}

func TestRecordFirstBootWrapsStoreErrors(t *testing.T) {
	settings := &fakeSettingsStore{err: errors.New("connection refused")}

	_, err := NewService(settings, clock.Fixed{Time: firstInstant}).RecordFirstBoot(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "recording first boot time")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestRecordFirstBootRejectsCorruptStoredValue(t *testing.T) {
	settings := &fakeSettingsStore{values: map[string][]byte{firstBootKey: []byte(`"not a time"`)}}

	_, err := NewService(settings, clock.Fixed{Time: firstInstant}).RecordFirstBoot(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding stored first boot time")
}
