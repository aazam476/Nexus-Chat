// Package mail sends plain-text email: verification links, password resets
// and admin warnings.
//
// [SMTPSender] talks to the admin's mail server. Until the admin configures
// SMTP, [Disabled] is used and every send fails with [ErrNotConfigured], so
// features can tell the user why no email arrived. Start reading at
// [Message] and [SMTPSender.Send].
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

// ErrNotConfigured means the instance has no SMTP server set up.
var ErrNotConfigured = errors.New("email is not set up on this instance; an admin can configure SMTP in the Admin panel")

// Message is one email.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender sends email.
type Sender interface {
	Send(ctx context.Context, message Message) error
}

// Disabled is the Sender used while SMTP is not configured.
type Disabled struct{}

// Send always fails with ErrNotConfigured.
func (Disabled) Send(context.Context, Message) error {
	return ErrNotConfigured
}

// format renders the message as RFC 5322 text. The body is quoted-printable
// so any line length and character set is safe on every mail server.
func format(from string, message Message, now time.Time) ([]byte, error) {
	if err := checkAddress(from); err != nil {
		return nil, fmt.Errorf("sender address: %w", err)
	}
	if err := checkAddress(message.To); err != nil {
		return nil, fmt.Errorf("recipient address: %w", err)
	}
	if strings.ContainsAny(message.Subject, "\r\n") {
		return nil, errors.New("subject must be a single line")
	}
	var buffer bytes.Buffer
	headers := []string{
		"From: " + from,
		"To: " + message.To,
		"Subject: " + mime.QEncoding.Encode("utf-8", message.Subject),
		"Date: " + now.Format(time.RFC1123Z),
		"Message-ID: <" + randomID() + "@nexus-chat>",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
	}
	buffer.WriteString(strings.Join(headers, "\r\n") + "\r\n\r\n")
	body := quotedprintable.NewWriter(&buffer)
	_, _ = body.Write([]byte(message.Text)) // writes to a bytes.Buffer cannot fail
	_ = body.Close()
	return buffer.Bytes(), nil
}

// checkAddress accepts a bare address only. Rejecting names, line breaks
// and commas prevents header injection and sending to extra recipients.
func checkAddress(address string) error {
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Address != address {
		return fmt.Errorf("%q is not a single plain email address", address)
	}
	return nil
}

func randomID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes) // crypto/rand.Read never fails (Go 1.24+)
	return hex.EncodeToString(bytes)
}
