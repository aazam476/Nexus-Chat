package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// Security is how the connection to the mail server is protected.
type Security string

// Security options. STARTTLS (usually port 587) and implicit TLS (usually
// port 465) both encrypt; None is only for a relay on the same machine or
// network.
const (
	SecurityStartTLS Security = "starttls"
	SecurityTLS      Security = "tls"
	SecurityNone     Security = "none"
)

// dialTimeout bounds connecting to the mail server.
const dialTimeout = 15 * time.Second

// SMTPConfig is the admin's mail server settings.
type SMTPConfig struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Security Security `json:"security"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
}

// SMTPSender sends mail through an SMTP server.
type SMTPSender struct {
	config SMTPConfig
	clock  clock.Clock
}

// NewSMTPSender returns a Sender for config.
func NewSMTPSender(config SMTPConfig, clock clock.Clock) *SMTPSender {
	return &SMTPSender{config: config, clock: clock}
}

// Send delivers message. net/smtp's PlainAuth refuses to send the password
// over an unencrypted connection unless the server is on localhost.
func (sender *SMTPSender) Send(ctx context.Context, message Message) error {
	body, err := format(sender.config.From, message, sender.clock.Now())
	if err != nil {
		return err
	}
	client, err := sender.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }() // after Quit this is a no-op
	if sender.config.Username != "" {
		auth := smtp.PlainAuth("", sender.config.Username, sender.config.Password, sender.config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("logging in to mail server: %w", err)
		}
	}
	if err := deliver(client, sender.config.From, message.To, body); err != nil {
		return err
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("closing mail connection: %w", err)
	}
	return nil
}

func (sender *SMTPSender) connect(ctx context.Context) (*smtp.Client, error) {
	address := net.JoinHostPort(sender.config.Host, strconv.Itoa(sender.config.Port))
	tlsConfig := &tls.Config{ServerName: sender.config.Host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: dialTimeout}
	var connection net.Conn
	var err error
	if sender.config.Security == SecurityTLS {
		connection, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to mail server %s: %w", address, err)
	}
	client, err := smtp.NewClient(connection, sender.config.Host)
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("talking to mail server: %w", err)
	}
	if sender.config.Security == SecurityStartTLS {
		if err := client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("starting TLS with mail server: %w", err)
		}
	}
	return client, nil
}

func deliver(client *smtp.Client, from string, to string, body []byte) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail server refused sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("mail server refused recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("starting message: %w", err)
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("sending message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finishing message: %w", err)
	}
	return nil
}
