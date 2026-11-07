package setup

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/setup/store"
)

type fakeStore struct {
	complete      bool
	completeError error
	readError     error
	stored        map[string]string
	saved         []store.Completion
}

func (fake *fakeStore) IsComplete(context.Context) (bool, error) {
	return fake.complete, fake.readError
}

func (fake *fakeStore) StoredString(_ context.Context, key string) (string, error) {
	return fake.stored[key], nil
}

func (fake *fakeStore) Complete(_ context.Context, completion store.Completion) error {
	if fake.completeError != nil {
		return fake.completeError
	}
	fake.saved = append(fake.saved, completion)
	fake.complete = true
	return nil
}

type fakeHasher struct{ err error }

func (fake fakeHasher) Hash(_ context.Context, password string) (string, error) {
	return "hashed:" + password, fake.err
}

type fakeLimiter struct{ deny bool }

func (fake fakeLimiter) Allow(string) bool { return !fake.deny }

var testTime = time.Date(2026, 11, 7, 12, 0, 0, 123456789, time.UTC)

// started returns a started Service with fakes, and the token it logged.
func started(t *testing.T, fake *fakeStore, change func(*Dependencies)) (*Service, string) {
	t.Helper()
	var logs bytes.Buffer
	dependencies := Dependencies{
		Store: fake, Hasher: fakeHasher{}, Limiter: fakeLimiter{},
		Clock: clock.Fixed{Time: testTime}, Logger: slog.New(slog.NewTextHandler(&logs, nil)),
	}
	if change != nil {
		change(&dependencies)
	}
	service := NewService(dependencies)
	require.NoError(t, service.Start(t.Context()))
	match := regexp.MustCompile(`setup_token=([A-Z2-7-]+)`).FindStringSubmatch(logs.String())
	if match == nil {
		return service, ""
	}
	return service, match[1]
}

func validRequest(token string) Request {
	return Request{
		Token:            token,
		Admin:            accounts.NewAccount{Username: "admin", DisplayName: "Admin", Email: "Admin@Example.com", Password: "a long password"},
		InstanceName:     "Example Chat",
		Domain:           "chat.example.com",
		RegistrationMode: "invite_only",
	}
}

var errBoom = errors.New("boom")

func errAlreadyCompleteFromStore() error { return store.ErrAlreadyComplete }
