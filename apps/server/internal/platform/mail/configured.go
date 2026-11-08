package mail

import (
	"context"
	"fmt"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// ConfigSource returns the current SMTP settings, and false while SMTP is
// not configured.
type ConfigSource func(ctx context.Context) (SMTPConfig, bool, error)

// Configured sends mail with whatever SMTP settings are current, so an
// admin's change in the Admin panel applies to the very next email.
type Configured struct {
	source ConfigSource
	clock  clock.Clock
}

// NewConfigured returns a Sender reading its settings from source.
func NewConfigured(source ConfigSource, clock clock.Clock) *Configured {
	return &Configured{source: source, clock: clock}
}

// Send sends message, or returns ErrNotConfigured. A problem talking to
// the mail server is returned as a *DeliveryError.
func (configured *Configured) Send(ctx context.Context, message Message) error {
	config, found, err := configured.source(ctx)
	if err != nil {
		return fmt.Errorf("reading SMTP settings: %w", err)
	}
	if !found || config.Host == "" {
		return ErrNotConfigured
	}
	if err := NewSMTPSender(config, configured.clock).Send(ctx, message); err != nil {
		return &DeliveryError{Err: err}
	}
	return nil
}

// DeliveryError means the mail server could not be reached or refused the
// message. Its message is meant for the admin who configured the server.
type DeliveryError struct {
	Err error
}

func (delivery *DeliveryError) Error() string { return delivery.Err.Error() }

// Unwrap returns the underlying error.
func (delivery *DeliveryError) Unwrap() error { return delivery.Err }
