package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/ratelimit"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/secretbox"
)

// ErrTestRateLimited means an admin sent too many test emails.
var ErrTestRateLimited = errors.New("too many test emails; wait a minute")

// SettingsStore reads and writes instance settings.
type SettingsStore interface {
	Load(ctx context.Context, key string, target any) (bool, error)
	Save(ctx context.Context, key string, value any) error
	InstanceName(ctx context.Context) (string, error)
}

// passwordContext binds the sealed SMTP password to this one setting.
//
//nolint:gosec // G101: a context label for encryption, not a credential.
const passwordContext = "nexus-chat smtp password"

// SMTPUpdate is an admin's change to the mail server settings. A nil
// Password keeps the stored one.
type SMTPUpdate struct {
	Host     string
	Port     int
	Security mail.Security
	Username string
	Password *string
	From     string
}

// SMTPView is what the Admin panel shows: the settings without the password.
type SMTPView struct {
	Host        string
	Port        int
	Security    mail.Security
	Username    string
	From        string
	PasswordSet bool
}

// storedSMTP is the JSON saved under instanceconfig.KeySMTP. The password
// is kept only encrypted, so a database dump does not reveal it.
type storedSMTP struct {
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	Security       mail.Security `json:"security"`
	Username       string        `json:"username"`
	From           string        `json:"from"`
	SealedPassword []byte        `json:"sealedPassword,omitempty"`
}

// MailSettings manages the SMTP settings and sends mail with them.
type MailSettings struct {
	settings SettingsStore
	box      *secretbox.Box
	sender   mail.Sender
	limiter  *ratelimit.Limiter
}

// MailDependencies configure NewMailSettings.
type MailDependencies struct {
	Settings SettingsStore
	// Box encrypts the stored password.
	Box *secretbox.Box
	// NewSender builds the Sender for every email the server sends from a
	// source of the current settings; mail.NewConfigured in production.
	NewSender func(source mail.ConfigSource) mail.Sender
	// TestLimiter bounds test emails per admin.
	TestLimiter *ratelimit.Limiter
}

// NewMailSettings returns a MailSettings.
func NewMailSettings(dependencies MailDependencies) *MailSettings {
	service := &MailSettings{settings: dependencies.Settings, box: dependencies.Box, limiter: dependencies.TestLimiter}
	service.sender = dependencies.NewSender(service.Current)
	return service
}

// Sender sends email with whatever settings are current when it sends.
func (service *MailSettings) Sender() mail.Sender {
	return service.sender
}

// Current returns the settings to send with, password decrypted. It is a
// mail.ConfigSource; configured is false until a mail server is set.
func (service *MailSettings) Current(ctx context.Context) (mail.SMTPConfig, bool, error) {
	stored, err := service.load(ctx)
	if err != nil {
		return mail.SMTPConfig{}, false, err
	}
	config := mail.SMTPConfig{
		Host: stored.Host, Port: stored.Port, Security: stored.Security, Username: stored.Username, From: stored.From,
	}
	if stored.SealedPassword != nil {
		password, err := service.box.Open(passwordContext, stored.SealedPassword)
		if err != nil {
			return mail.SMTPConfig{}, false, fmt.Errorf("SMTP password: %w", err)
		}
		config.Password = string(password)
	}
	return config, config.Host != "", nil
}

// View returns the stored settings without the password, or the usual
// defaults if none are stored.
func (service *MailSettings) View(ctx context.Context) (SMTPView, error) {
	stored, err := service.load(ctx)
	if err != nil {
		return SMTPView{}, err
	}
	return view(stored), nil
}

func (service *MailSettings) load(ctx context.Context) (storedSMTP, error) {
	stored := storedSMTP{Port: defaultSubmissionPort, Security: mail.SecurityStartTLS}
	if _, err := service.settings.Load(ctx, instanceconfig.KeySMTP, &stored); err != nil {
		return storedSMTP{}, err
	}
	return stored, nil
}

// Update validates and saves update. Keeping the old password needs no
// decryption, so an admin can always fix settings whose password no
// longer opens by entering it again.
func (service *MailSettings) Update(ctx context.Context, update SMTPUpdate) (SMTPView, error) {
	current, err := service.load(ctx)
	if err != nil {
		return SMTPView{}, err
	}
	stored := storedSMTP{
		Host: strings.TrimSpace(update.Host), Port: update.Port, Security: update.Security,
		Username: update.Username, From: strings.TrimSpace(update.From), SealedPassword: current.SealedPassword,
	}
	if err := check(stored, update.Password); err != nil {
		return SMTPView{}, err
	}
	if update.Password != nil {
		stored.SealedPassword = nil
		if *update.Password != "" {
			stored.SealedPassword = service.box.Seal(passwordContext, []byte(*update.Password))
		}
	}
	if err := service.settings.Save(ctx, instanceconfig.KeySMTP, stored); err != nil {
		return SMTPView{}, err
	}
	return view(stored), nil
}

func view(stored storedSMTP) SMTPView {
	return SMTPView{
		Host: stored.Host, Port: stored.Port, Security: stored.Security, Username: stored.Username,
		From: stored.From, PasswordSet: stored.SealedPassword != nil,
	}
}

// SendTest emails a test message to the admin at address. It returns
// mail.ErrNotConfigured before SMTP is set up, and a *mail.DeliveryError
// if the mail server fails.
func (service *MailSettings) SendTest(ctx context.Context, adminKey string, address string, displayName string) error {
	if !service.limiter.Allow(adminKey) {
		return ErrTestRateLimited
	}
	name, err := service.settings.InstanceName(ctx)
	if err != nil {
		return err
	}
	message, err := mail.Render(mail.TemplateTestEmail, address, mail.TemplateData{InstanceName: name, DisplayName: displayName})
	if err != nil {
		return err
	}
	return service.sender.Send(ctx, message)
}
