package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/ratelimit"
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

// fakeSettings keeps settings as JSON-compatible Go values. failOn names
// the method that fails.
type fakeSettings struct {
	smtp   *mail.SMTPConfig
	failOn string
}

func (fake *fakeSettings) Load(_ context.Context, _ string, target any) (bool, error) {
	if fake.failOn == "Load" {
		return false, errFailure
	}
	if fake.smtp == nil {
		return false, nil
	}
	*target.(*mail.SMTPConfig) = *fake.smtp //nolint:forcetypeassert // only SMTP is stored here
	return true, nil
}

func (fake *fakeSettings) Save(_ context.Context, _ string, value any) error {
	if fake.failOn == "Save" {
		return errFailure
	}
	saved := value.(mail.SMTPConfig) //nolint:forcetypeassert // only SMTP is stored here
	fake.smtp = &saved
	return nil
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

func newMailSettings(settings *fakeSettings, sender *fakeSender) *MailSettings {
	return NewMailSettings(settings, sender, ratelimit.New(ratelimit.PerMinute(3, 3), clock.Fixed{}))
}

func adminContext(t *testing.T) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New()})
}
