// Package emails sends the account emails that carry a single-use link:
// verifying an address and resetting a password.
//
// Links put the token after "#", which browsers never send to a server, so
// tokens cannot end up in web server or proxy logs. The client reads the
// token from the address bar and submits it to the API. Start reading at
// [Service.SendVerification].
package emails

import (
	"context"
	"fmt"
	"net/url"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
)

// kind describes one type of account email: which token it carries, which
// template it uses, and which client page opens its link.
type kind struct {
	purpose  tokens.Purpose
	template string
	path     string
}

// The two kinds of account email.
var (
	verificationEmail  = kind{purpose: tokens.VerifyEmail, template: mail.TemplateVerifyEmail, path: "/verify-email"}       //nolint:gochecknoglobals // constant description
	passwordResetEmail = kind{purpose: tokens.ResetPassword, template: mail.TemplateResetPassword, path: "/reset-password"} //nolint:gochecknoglobals // constant description
)

// tokenIssuer is satisfied by *tokens.Service.
type tokenIssuer interface {
	Issue(ctx context.Context, userID uuid.UUID, purpose tokens.Purpose) (string, error)
}

// instanceInfo is satisfied by *instanceconfig.Service.
type instanceInfo interface {
	BaseURL(ctx context.Context) (string, error)
	InstanceName(ctx context.Context) (string, error)
}

// Service sends account emails.
type Service struct {
	tokens   tokenIssuer
	sender   mail.Sender
	instance instanceInfo
}

// NewService returns a Service.
func NewService(issuer tokenIssuer, sender mail.Sender, instance instanceInfo) *Service {
	return &Service{tokens: issuer, sender: sender, instance: instance}
}

// SendVerification emails user a link that confirms their address.
func (service *Service) SendVerification(ctx context.Context, user store.User) error {
	return service.send(ctx, user, verificationEmail)
}

// SendPasswordReset emails user a link that lets them choose a new password.
func (service *Service) SendPasswordReset(ctx context.Context, user store.User) error {
	return service.send(ctx, user, passwordResetEmail)
}

func (service *Service) send(ctx context.Context, user store.User, email kind) error {
	base, err := service.instance.BaseURL(ctx)
	if err != nil {
		return err
	}
	name, err := service.instance.InstanceName(ctx)
	if err != nil {
		return err
	}
	token, err := service.tokens.Issue(ctx, user.ID, email.purpose)
	if err != nil {
		return err
	}
	link := base + email.path + "#token=" + url.QueryEscape(token)
	message, err := mail.Render(email.template, user.Email, mail.TemplateData{InstanceName: name, DisplayName: user.DisplayName, Link: link})
	if err != nil {
		return err
	}
	if err := service.sender.Send(ctx, message); err != nil {
		return fmt.Errorf("sending %s email: %w", email.template, err)
	}
	return nil
}

// verificationStore marks addresses as verified.
type verificationStore interface {
	MarkEmailVerified(ctx context.Context, arg store.MarkEmailVerifiedParams) error
}

// tokenRedeemer is satisfied by *tokens.Service.
type tokenRedeemer interface {
	Redeem(ctx context.Context, purpose tokens.Purpose, token string) (uuid.UUID, error)
}

// Verifier confirms email addresses from verification links.
type Verifier struct {
	tokens tokenRedeemer
	users  verificationStore
	clock  clock.Clock
}

// NewVerifier returns a Verifier.
func NewVerifier(redeemer tokenRedeemer, users verificationStore, clock clock.Clock) *Verifier {
	return &Verifier{tokens: redeemer, users: users, clock: clock}
}

// Verify redeems a verification token and marks its owner's email verified.
func (verifier *Verifier) Verify(ctx context.Context, token string) error {
	userID, err := verifier.tokens.Redeem(ctx, tokens.VerifyEmail, token)
	if err != nil {
		return err
	}
	verifiedAt := verifier.clock.Now()
	if err := verifier.users.MarkEmailVerified(ctx, store.MarkEmailVerifiedParams{ID: userID, EmailVerifiedAt: &verifiedAt}); err != nil {
		return fmt.Errorf("marking email verified: %w", err)
	}
	return nil
}
