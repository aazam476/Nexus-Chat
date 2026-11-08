package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/ratelimit"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// Limits on SMTP fields. A host name is at most 253 characters (RFC 1035).
const (
	maximumHostLength     = 253
	maximumUsernameLength = 320
	maximumPasswordLength = 1024
	maximumPort           = 65535
	defaultSubmissionPort = 587
)

var (
	// ErrTestRateLimited means an admin sent too many test emails.
	ErrTestRateLimited = errors.New("too many test emails; wait a minute")

	errHostRequired      = errors.New("is required")
	errHostInvalid       = fmt.Errorf("must be a host name or IP address of at most %d characters", maximumHostLength)
	errPortInvalid       = fmt.Errorf("must be between 1 and %d", maximumPort)
	errSecurityInvalid   = errors.New("must be starttls, tls or none")
	errPasswordUnsafe    = errors.New("a password is only sent over an encrypted connection; use starttls or tls, or a relay on localhost")
	errUsernameTooLong   = fmt.Errorf("must be at most %d characters", maximumUsernameLength)
	errPasswordTooLong   = fmt.Errorf("must be at most %d characters", maximumPasswordLength)
	errFromAddressFormat = errors.New("must be a plain email address such as chat@example.com")
)

// SettingsStore reads and writes instance settings.
type SettingsStore interface {
	Load(ctx context.Context, key string, target any) (bool, error)
	Save(ctx context.Context, key string, value any) error
	InstanceName(ctx context.Context) (string, error)
}

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

// MailSettings manages the SMTP settings.
type MailSettings struct {
	settings SettingsStore
	sender   mail.Sender
	limiter  *ratelimit.Limiter
}

// NewMailSettings returns a MailSettings saving to settings. sender must
// read the current settings for each message, as mail.Configured does, so
// a test email checks what was just saved. limiter bounds test emails per
// admin.
func NewMailSettings(settings SettingsStore, sender mail.Sender, limiter *ratelimit.Limiter) *MailSettings {
	return &MailSettings{settings: settings, sender: sender, limiter: limiter}
}

// Current returns the stored settings, or the usual defaults if none are
// stored. Configured reports whether a mail server is set.
func (service *MailSettings) Current(ctx context.Context) (mail.SMTPConfig, bool, error) {
	config := mail.SMTPConfig{Port: defaultSubmissionPort, Security: mail.SecurityStartTLS}
	if _, err := service.settings.Load(ctx, instanceconfig.KeySMTP, &config); err != nil {
		return mail.SMTPConfig{}, false, err
	}
	return config, config.Host != "", nil
}

// Update validates and saves update, returning the saved settings.
func (service *MailSettings) Update(ctx context.Context, update SMTPUpdate) (mail.SMTPConfig, error) {
	current, _, err := service.Current(ctx)
	if err != nil {
		return mail.SMTPConfig{}, err
	}
	config := mail.SMTPConfig{
		Host: strings.TrimSpace(update.Host), Port: update.Port, Security: update.Security,
		Username: update.Username, Password: current.Password, From: strings.TrimSpace(update.From),
	}
	if update.Password != nil {
		config.Password = *update.Password
	}
	if err := check(config); err != nil {
		return mail.SMTPConfig{}, err
	}
	if err := service.settings.Save(ctx, instanceconfig.KeySMTP, config); err != nil {
		return mail.SMTPConfig{}, err
	}
	return config, nil
}

// SendTest emails a test message to the admin at address. It returns
// mail.ErrNotConfigured before SMTP is set up, and the mail server's
// error if delivery fails.
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

func check(config mail.SMTPConfig) error {
	var problems validation.Errors
	problems.Add("host", checkHost(config.Host))
	if config.Port < 1 || config.Port > maximumPort {
		problems.Add("port", errPortInvalid)
	}
	switch config.Security {
	case mail.SecurityStartTLS, mail.SecurityTLS:
	case mail.SecurityNone:
		// net/smtp refuses to send a password in the clear to any other
		// host, so saving such settings would only fail later.
		if config.Password != "" && !isLoopback(config.Host) {
			problems.Add("security", errPasswordUnsafe)
		}
	default:
		problems.Add("security", errSecurityInvalid)
	}
	if len(config.Username) > maximumUsernameLength {
		problems.Add("username", errUsernameTooLong)
	}
	if len(config.Password) > maximumPasswordLength {
		problems.Add("password", errPasswordTooLong)
	}
	if mail.CheckAddress(config.From) != nil {
		problems.Add("from", errFromAddressFormat)
	}
	return problems.Err()
}

func checkHost(host string) error {
	if host == "" {
		return errHostRequired
	}
	if len(host) > maximumHostLength || strings.ContainsFunc(host, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || r == '/'
	}) {
		return errHostInvalid
	}
	return nil
}

// isLoopback matches the hosts net/smtp.PlainAuth accepts without TLS.
func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
