// Package store saves the result of setup in one database transaction.
//
// It combines the generated account and settings queries; it holds no SQL
// of its own.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	settingsstore "github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
)

// ErrAlreadyComplete means another request finished setup first.
var ErrAlreadyComplete = errors.New("setup is already complete")

// Completion is everything a successful setup saves.
type Completion struct {
	Admin       accountsstore.InsertUserParams
	Settings    map[string]string
	CompletedAt time.Time
}

// Store reads and writes setup state.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store using pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// IsComplete reports whether setup has finished.
func (store *Store) IsComplete(ctx context.Context) (bool, error) {
	_, err := settingsstore.New(store.pool).GetSetting(ctx, instanceconfig.KeySetupCompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading setup state: %w", err)
	}
	return true, nil
}

// StoredString returns a JSON-string setting, or "" if it is not set.
func (store *Store) StoredString(ctx context.Context, key string) (string, error) {
	setting, err := settingsstore.New(store.pool).GetSetting(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading setting %s: %w", key, err)
	}
	var value string
	if err := json.Unmarshal(setting.Value, &value); err != nil {
		return "", fmt.Errorf("decoding setting %s: %w", key, err)
	}
	return value, nil
}

// Complete saves the admin account and settings, and marks setup complete,
// all in one transaction. If setup was completed concurrently it returns
// ErrAlreadyComplete and saves nothing.
func (store *Store) Complete(ctx context.Context, completion Completion) error {
	return database.InTransaction(ctx, store.pool, func(transaction pgx.Tx) error {
		settings := settingsstore.New(transaction)
		completedAt, _ := json.Marshal(completion.CompletedAt) // a time.Time always encodes
		inserted, err := settings.InsertSettingIfAbsent(ctx, settingsstore.InsertSettingIfAbsentParams{
			Key: instanceconfig.KeySetupCompletedAt, Value: completedAt,
		})
		if err != nil {
			return fmt.Errorf("marking setup complete: %w", err)
		}
		if inserted == 0 {
			return ErrAlreadyComplete
		}
		if err := accountsstore.New(transaction).InsertUser(ctx, completion.Admin); err != nil {
			return fmt.Errorf("creating admin account: %w", err)
		}
		return saveSettings(ctx, settings, completion.Settings)
	})
}

func saveSettings(ctx context.Context, settings *settingsstore.Queries, values map[string]string) error {
	for key, value := range values {
		encoded, _ := json.Marshal(value) // a string always encodes
		if err := settings.UpsertSetting(ctx, settingsstore.UpsertSettingParams{Key: key, Value: encoded}); err != nil {
			return fmt.Errorf("saving setting %s: %w", key, err)
		}
	}
	return nil
}
