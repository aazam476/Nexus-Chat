package admin

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig"
)

// keyCertificateNotice records the last expiry email sent, so a restart
// does not send it again.
const keyCertificateNotice = "tls.expiry_notice"

// sentNotice is what keyCertificateNotice holds: which certificate (by its
// expiry time) and the smallest threshold already emailed about.
type sentNotice struct {
	NotAfter  time.Time     `json:"notAfter"`
	Threshold time.Duration `json:"threshold"`
}

// CertificateNotices emails the admin (ADMIN_EMAIL) when the HTTPS
// certificate comes within 30, 7 and 1 days of expiry (spec §3.3).
type CertificateNotices struct {
	settings   SettingsStore
	mailer     mail.Sender
	adminEmail string
	logger     *slog.Logger
}

// NewCertificateNotices returns the notifier; pass Notify to
// tlsconfig.Manager.OnExpiring.
func NewCertificateNotices(settings SettingsStore, mailer mail.Sender, adminEmail string, logger *slog.Logger) *CertificateNotices {
	return &CertificateNotices{settings: settings, mailer: mailer, adminEmail: adminEmail, logger: logger}
}

// Notify sends the email for expiry unless it was already sent for this
// certificate and threshold. Failures are logged: the log warning the
// manager wrote already tells the admin, and the next restart tries again.
func (notices *CertificateNotices) Notify(ctx context.Context, expiry tlsconfig.Expiry) {
	if err := notices.notify(ctx, expiry); err != nil {
		notices.logger.WarnContext(ctx, "could not email the admin about the expiring HTTPS certificate", slog.Any("error", err))
	}
}

func (notices *CertificateNotices) notify(ctx context.Context, expiry tlsconfig.Expiry) error {
	var sent sentNotice
	if _, err := notices.settings.Load(ctx, keyCertificateNotice, &sent); err != nil {
		return fmt.Errorf("reading the last notice: %w", err)
	}
	if sent.NotAfter.Equal(expiry.Certificate.NotAfter) && sent.Threshold <= expiry.Threshold {
		return nil
	}
	instanceName, err := notices.settings.InstanceName(ctx)
	if err != nil {
		return fmt.Errorf("reading the instance name: %w", err)
	}
	message, err := mail.Render(mail.TemplateCertificateExpiring, notices.adminEmail, mail.TemplateData{
		InstanceName: instanceName,
		Domain:       strings.Join(expiry.Certificate.Names, ", "),
		Expires:      expiry.Certificate.NotAfter.UTC().Format("2 January 2006 at 15:04 UTC"),
		DaysLeft:     timeLeft(expiry.Remaining),
	})
	if err != nil {
		return err
	}
	if err := notices.mailer.Send(ctx, message); err != nil {
		return fmt.Errorf("sending: %w", err)
	}
	return notices.settings.Save(ctx, keyCertificateNotice, sentNotice{NotAfter: expiry.Certificate.NotAfter, Threshold: expiry.Threshold})
}

// timeLeft puts a duration in words, rounded down to whole days.
func timeLeft(remaining time.Duration) string {
	days := int(remaining / (24 * time.Hour))
	switch {
	case remaining <= 0:
		return "less than a day (it may already have expired)"
	case days == 0:
		return "less than a day"
	case days == 1:
		return "1 day"
	default:
		return fmt.Sprintf("%d days", days)
	}
}
