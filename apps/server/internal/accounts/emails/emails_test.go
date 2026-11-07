package emails

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
)

type fakeIssuer struct {
	purposes []tokens.Purpose
	err      error
}

func (fake *fakeIssuer) Issue(_ context.Context, _ uuid.UUID, purpose tokens.Purpose) (string, error) {
	fake.purposes = append(fake.purposes, purpose)
	return "token+with/odd=chars", fake.err
}

type outbox struct {
	sent []mail.Message
	err  error
}

func (box *outbox) Send(_ context.Context, message mail.Message) error {
	box.sent = append(box.sent, message)
	return box.err
}

type instance struct {
	baseErr error
	nameErr error
}

func (fake instance) BaseURL(context.Context) (string, error) {
	return "https://chat.example.com", fake.baseErr
}
func (fake instance) InstanceName(context.Context) (string, error) {
	return "Example Chat", fake.nameErr
}

var alice = store.User{ID: uuid.New(), DisplayName: "Alice", Email: "alice@example.com"}

func TestSendVerificationAndReset(t *testing.T) {
	issuer := &fakeIssuer{}
	box := &outbox{}
	service := NewService(issuer, box, instance{})

	require.NoError(t, service.SendVerification(t.Context(), alice))
	require.NoError(t, service.SendPasswordReset(t.Context(), alice))

	assert.Equal(t, []tokens.Purpose{tokens.VerifyEmail, tokens.ResetPassword}, issuer.purposes)
	require.Len(t, box.sent, 2)
	assert.Contains(t, box.sent[0].Text, "https://chat.example.com/verify-email#token=token%2Bwith%2Fodd%3Dchars")
	assert.Contains(t, box.sent[1].Text, "https://chat.example.com/reset-password#token=")
	assert.True(t, strings.HasPrefix(box.sent[1].Subject, "Reset your password"))
	assert.Equal(t, "alice@example.com", box.sent[0].To)
}

func TestSendFailures(t *testing.T) {
	failure := errors.New("failure")
	tests := map[string]struct {
		issuer   *fakeIssuer
		box      *outbox
		instance instance
	}{
		"no domain":     {issuer: &fakeIssuer{}, box: &outbox{}, instance: instance{baseErr: failure}},
		"settings down": {issuer: &fakeIssuer{}, box: &outbox{}, instance: instance{nameErr: failure}},
		"token fails":   {issuer: &fakeIssuer{err: failure}, box: &outbox{}},
		"mail fails":    {issuer: &fakeIssuer{}, box: &outbox{err: failure}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := NewService(test.issuer, test.box, test.instance).SendVerification(t.Context(), alice)

			assert.ErrorIs(t, err, failure)
		})
	}
}

type fakeRedeemer struct {
	user uuid.UUID
	err  error
}

func (fake fakeRedeemer) Redeem(_ context.Context, purpose tokens.Purpose, _ string) (uuid.UUID, error) {
	if purpose != tokens.VerifyEmail {
		return uuid.Nil, errors.New("wrong purpose")
	}
	return fake.user, fake.err
}

type fakeVerifications struct {
	marked []store.MarkEmailVerifiedParams
	err    error
}

func (fake *fakeVerifications) MarkEmailVerified(_ context.Context, arg store.MarkEmailVerifiedParams) error {
	fake.marked = append(fake.marked, arg)
	return fake.err
}

func TestVerify(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	users := &fakeVerifications{}

	require.NoError(t, NewVerifier(fakeRedeemer{user: alice.ID}, users, clock.Fixed{Time: now}).Verify(t.Context(), "token"))

	require.Len(t, users.marked, 1)
	assert.Equal(t, alice.ID, users.marked[0].ID)
	assert.Equal(t, now, *users.marked[0].EmailVerifiedAt)
}

func TestVerifyFailures(t *testing.T) {
	failure := errors.New("failure")

	err := NewVerifier(fakeRedeemer{err: tokens.ErrInvalidToken}, &fakeVerifications{}, clock.Fixed{}).Verify(t.Context(), "x")
	require.ErrorIs(t, err, tokens.ErrInvalidToken)

	err = NewVerifier(fakeRedeemer{user: alice.ID}, &fakeVerifications{err: failure}, clock.Fixed{}).Verify(t.Context(), "x")
	assert.ErrorIs(t, err, failure)
}
