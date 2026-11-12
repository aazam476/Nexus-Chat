package instanceconfig

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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

func (fake *fakeSettingsStore) GetSetting(_ context.Context, key string) (store.InstanceSetting, error) {
	if fake.err != nil {
		return store.InstanceSetting{}, fake.err
	}
	value, ok := fake.values[key]
	if !ok {
		return store.InstanceSetting{}, pgx.ErrNoRows
	}
	return store.InstanceSetting{Key: key, Value: value}, nil
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

func (fake *fakeSettingsStore) UpsertSetting(_ context.Context, arg store.UpsertSettingParams) error {
	if fake.err != nil {
		return fake.err
	}
	fake.values[arg.Key] = arg.Value
	return nil
}

var firstInstant = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func TestRecordFirstBootStoresTheCurrentTimeOnce(t *testing.T) {
	settings := &fakeSettingsStore{values: map[string][]byte{}}

	first, err := NewService(settings, clock.Fixed{Time: firstInstant}, "").RecordFirstBoot(context.Background())
	require.NoError(t, err)
	later, err := NewService(settings, clock.Fixed{Time: firstInstant.Add(time.Hour)}, "").RecordFirstBoot(context.Background())
	require.NoError(t, err)

	assert.Equal(t, firstInstant, first)
	assert.Equal(t, firstInstant, later, "a later boot must not overwrite the first boot time")
}

func TestRecordFirstBootWrapsStoreErrors(t *testing.T) {
	settings := &fakeSettingsStore{err: errors.New("connection refused")}

	_, err := NewService(settings, clock.Fixed{Time: firstInstant}, "").RecordFirstBoot(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "recording first boot time")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestRecordFirstBootRejectsCorruptStoredValue(t *testing.T) {
	settings := &fakeSettingsStore{values: map[string][]byte{firstBootKey: []byte(`"not a time"`)}}

	_, err := NewService(settings, clock.Fixed{Time: firstInstant}, "").RecordFirstBoot(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding stored first boot time")
}

func TestDomainAndBaseURL(t *testing.T) {
	stored := &fakeSettingsStore{values: map[string][]byte{KeyInstanceDomain: []byte(`"stored.example.com"`)}}
	empty := &fakeSettingsStore{values: map[string][]byte{}}

	domain, err := NewService(stored, clock.Fixed{}, "").Domain(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "stored.example.com", domain)

	base, err := NewService(stored, clock.Fixed{}, "env.example.com").BaseURL(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "https://env.example.com", base, "DOMAIN from .env wins")

	_, err = NewService(empty, clock.Fixed{}, "").BaseURL(t.Context())
	require.ErrorIs(t, err, ErrNoDomain)

	_, err = NewService(&fakeSettingsStore{err: errors.New("down")}, clock.Fixed{}, "").BaseURL(t.Context())
	assert.ErrorContains(t, err, "down")
}

func TestRegistrationMode(t *testing.T) {
	mode, err := NewService(&fakeSettingsStore{values: map[string][]byte{}}, clock.Fixed{}, "").RegistrationMode(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "invite_only", mode, "the safe default")

	open := &fakeSettingsStore{values: map[string][]byte{KeyRegistrationMode: []byte(`"open"`)}}
	mode, err = NewService(open, clock.Fixed{}, "").RegistrationMode(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "open", mode)
}

func TestLoadRejectsCorruptValues(t *testing.T) {
	corrupt := &fakeSettingsStore{values: map[string][]byte{"some.key": []byte(`{`)}}
	var target map[string]any

	_, err := NewService(corrupt, clock.Fixed{}, "").Load(t.Context(), "some.key", &target)

	assert.ErrorContains(t, err, "decoding setting some.key")
}

func TestInstanceName(t *testing.T) {
	name, err := NewService(&fakeSettingsStore{values: map[string][]byte{}}, clock.Fixed{}, "").InstanceName(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "Nexus Chat", name)

	named := &fakeSettingsStore{values: map[string][]byte{KeyInstanceName: []byte(`"Example"`)}}
	name, err = NewService(named, clock.Fixed{}, "").InstanceName(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "Example", name)
}

func TestSaveReplacesTheStoredValue(t *testing.T) {
	settings := &fakeSettingsStore{values: map[string][]byte{KeyInstanceName: []byte(`"Old"`)}}
	service := NewService(settings, clock.Fixed{}, "")

	require.NoError(t, service.Save(t.Context(), KeyInstanceName, "New"))

	name, err := service.InstanceName(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "New", name)
}

func TestSaveReportsFailures(t *testing.T) {
	service := NewService(&fakeSettingsStore{values: map[string][]byte{}}, clock.Fixed{}, "")
	require.ErrorContains(t, service.Save(t.Context(), "bad", func() {}), "encoding setting bad")

	failing := NewService(&fakeSettingsStore{err: errors.New("down")}, clock.Fixed{}, "")
	assert.ErrorContains(t, failing.Save(t.Context(), "key", "value"), "saving setting key: down")
}

func TestAttachmentLimit(t *testing.T) {
	service := NewService(&fakeSettingsStore{values: map[string][]byte{}}, clock.Fixed{}, "")
	limit, err := service.AttachmentMaxBytes(t.Context())
	require.NoError(t, err)
	assert.Equal(t, DefaultAttachmentMaxBytes, limit, "25 MiB until the admin changes it")

	require.NoError(t, service.SetAttachmentMaxBytes(t.Context(), 100<<20))
	limit, err = service.AttachmentMaxBytes(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(100<<20), limit)
	require.ErrorIs(t, service.SetAttachmentMaxBytes(t.Context(), MinimumAttachmentMaxBytes-1), ErrInvalidAttachmentLimit)
	require.ErrorIs(t, service.SetAttachmentMaxBytes(t.Context(), MaximumAttachmentMaxBytes+1), ErrInvalidAttachmentLimit)
	require.NoError(t, service.SetAttachmentMaxBytes(t.Context(), MaximumAttachmentMaxBytes))
}

func TestLinkPreviewProxy(t *testing.T) {
	service := NewService(&fakeSettingsStore{values: map[string][]byte{}}, clock.Fixed{}, "")
	enabled, err := service.LinkPreviewProxy(t.Context())
	require.NoError(t, err)
	assert.False(t, enabled, "off until the admin turns it on")
	require.NoError(t, service.SetLinkPreviewProxy(t.Context(), true))
	enabled, err = service.LinkPreviewProxy(t.Context())
	require.NoError(t, err)
	assert.True(t, enabled)
}
