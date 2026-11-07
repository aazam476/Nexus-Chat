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

// Send sends message, or returns ErrNotConfigured.
func (configured *Configured) Send(ctx context.Context, message Message) error {
	config, found, err := configured.source(ctx)
	if err != nil {
		return fmt.Errorf("reading SMTP settings: %w", err)
	}
	if !found || config.Host == "" {
		return ErrNotConfigured
	}
	return NewSMTPSender(config, configured.clock).Send(ctx, message)
}
