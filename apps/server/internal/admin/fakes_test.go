package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/ratelimit"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/secretbox"
	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig"
)

var errFailure = errors.New("database down")

type fakeUsers struct {
	admin bool
	err   error
}

func (fake fakeUsers) GetUserByID(_ context.Context, id uuid.UUID) (accountsstore.User, error) {
	return accountsstore.User{ID: id, Email: "ada@example.com", DisplayName: "Ada", IsInstanceAdmin: fake.admin}, fake.err
}

// fakeSettings stores the SMTP setting as JSON, like the database does.
// failOn names the method that fails.
type fakeSettings struct {
	smtp   []byte
	failOn string
}

func (fake *fakeSettings) Load(_ context.Context, _ string, target any) (bool, error) {
	if fake.failOn == "Load" {
		return false, errFailure
	}
	if fake.smtp == nil {
		return false, nil
	}
	return true, json.Unmarshal(fake.smtp, target)
}

func (fake *fakeSettings) Save(_ context.Context, _ string, value any) error {
	if fake.failOn == "Save" {
		return errFailure
	}
	encoded, err := json.Marshal(value)
	fake.smtp = encoded
	return err
}

func (fake *fakeSettings) InstanceName(context.Context) (string, error) {
	if fake.failOn == "InstanceName" {
		return "", errFailure
	}
	return "Example Chat", nil
}

type fakeSender struct {
	sent []mail.Message
	err  error
}

func (fake *fakeSender) Send(_ context.Context, message mail.Message) error {
	if fake.err != nil {
		return fake.err
	}
	fake.sent = append(fake.sent, message)
	return nil
}

type fakeCertificates struct {
	status     tlsconfig.Status
	installErr error
	removeErr  error
	installed  []string
}

func (fake *fakeCertificates) Status() tlsconfig.Status { return fake.status }

func (fake *fakeCertificates) Install(_ context.Context, certificatePEM []byte, keyPEM []byte) (tlsconfig.Info, error) {
	if fake.installErr != nil {
		return tlsconfig.Info{}, fake.installErr
	}
	fake.installed = append(fake.installed, string(certificatePEM), string(keyPEM))
	fake.status = tlsconfig.Status{Active: true, Info: tlsconfig.Info{Names: []string{"chat.example.com"}, Issuer: "Test CA"}}
	return fake.status.Info, nil
}

func (fake *fakeCertificates) Remove() error {
	if fake.removeErr != nil {
		return fake.removeErr
	}
	fake.status = tlsconfig.Status{}
	return nil
}

func testBox(t *testing.T) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(bytes.Repeat([]byte{7}, 32))
	require.NoError(t, err)
	return box
}

func newMailSettings(t *testing.T, settings *fakeSettings, sender *fakeSender) *MailSettings {
	t.Helper()
	return NewMailSettings(MailDependencies{
		Settings: settings, Box: testBox(t),
		NewSender:   func(mail.ConfigSource) mail.Sender { return sender },
		TestLimiter: ratelimit.New(ratelimit.PerMinute(3, 3), clock.Fixed{}),
	})
}

func adminContext(t *testing.T) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New()})
}
