package mail

import (
	"bufio"
	"context"
	"io"
	"mime/quotedprintable"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// fakeSMTPServer speaks just enough SMTP for net/smtp's client. It rejects
// the recipient "refused@example.com" and records the last message.
type fakeSMTPServer struct {
	listener net.Listener
	mutex    sync.Mutex
	commands []string
	data     string
}

func startFakeServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &fakeSMTPServer{listener: listener}
	// This goroutine ends when the listener is closed by the cleanup.
	go server.serve()
	t.Cleanup(func() { _ = listener.Close() })
	return server
}

func (server *fakeSMTPServer) port() int {
	return server.listener.Addr().(*net.TCPAddr).Port
}

func (server *fakeSMTPServer) serve() {
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			return
		}
		server.handle(connection)
	}
}

func (server *fakeSMTPServer) handle(connection net.Conn) {
	defer func() { _ = connection.Close() }()
	reader := bufio.NewReader(connection)
	reply := func(line string) { _, _ = connection.Write([]byte(line + "\r\n")) }
	reply("220 fake ESMTP")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimRight(line, "\r\n")
		server.record(command)
		switch upper := strings.ToUpper(command); {
		case strings.HasPrefix(upper, "EHLO"):
			reply("250-fake\r\n250 AUTH PLAIN")
		case strings.HasPrefix(upper, "AUTH"):
			reply("235 ok")
		case strings.HasPrefix(upper, "RCPT") && strings.Contains(command, "refused@"):
			reply("550 no such user")
		case strings.HasPrefix(upper, "DATA"):
			reply("354 go ahead")
			server.readData(reader)
			reply("250 queued")
		case strings.HasPrefix(upper, "QUIT"):
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

func (server *fakeSMTPServer) readData(reader *bufio.Reader) {
	var data strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == ".\r\n" {
			break
		}
		data.WriteString(line)
	}
	server.mutex.Lock()
	server.data = data.String()
	server.mutex.Unlock()
}

func (server *fakeSMTPServer) record(command string) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.commands = append(server.commands, command)
}

func (server *fakeSMTPServer) snapshot() ([]string, string) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return append([]string{}, server.commands...), server.data
}

var sentAt = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func sender(server *fakeSMTPServer, security Security, username string) *SMTPSender {
	return NewSMTPSender(SMTPConfig{
		Host: "127.0.0.1", Port: server.port(), Security: security,
		Username: username, Password: "secret", From: "chat@example.com",
	}, clock.Fixed{Time: sentAt})
}

func TestSendDeliversAMessage(t *testing.T) {
	server := startFakeServer(t)
	message := Message{To: "alice@example.com", Subject: "Bestätige deine E-Mail", Text: "Open this link:\nhttps://chat.example.com/verify?token=abc\n"}

	require.NoError(t, sender(server, SecurityNone, "chat").Send(t.Context(), message))

	commands, data := server.snapshot()
	assert.Contains(t, commands, "MAIL FROM:<chat@example.com>")
	assert.Contains(t, commands, "RCPT TO:<alice@example.com>")
	assert.Contains(t, strings.Join(commands, "\n"), "AUTH PLAIN")
	assert.Contains(t, data, "Subject: =?utf-8?q?Best=C3=A4tige_deine_E-Mail?=\r\n")
	assert.Contains(t, data, "Date: Sat, 07 Nov 2026 12:00:00 +0000\r\n")
	body := data[strings.Index(data, "\r\n\r\n")+4:]
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
	require.NoError(t, err)
	assert.Contains(t, string(decoded), "https://chat.example.com/verify?token=abc")
}

func TestSendWithoutLogin(t *testing.T) {
	server := startFakeServer(t)

	require.NoError(t, sender(server, SecurityNone, "").Send(t.Context(), Message{To: "a@example.com", Subject: "Hi", Text: "Hello"}))

	commands, _ := server.snapshot()
	assert.NotContains(t, strings.Join(commands, "\n"), "AUTH")
}

func TestSendFailures(t *testing.T) {
	server := startFakeServer(t)
	message := Message{To: "a@example.com", Subject: "Hi", Text: "Hello"}

	err := sender(server, SecurityNone, "").Send(t.Context(), Message{To: "refused@example.com", Subject: "Hi", Text: "x"})
	require.ErrorContains(t, err, "refused recipient")

	err = sender(server, SecurityStartTLS, "").Send(t.Context(), message)
	require.ErrorContains(t, err, "starting TLS", "the fake server does not offer STARTTLS")

	err = sender(server, SecurityTLS, "").Send(t.Context(), message)
	require.ErrorContains(t, err, "connecting to mail server", "a TLS handshake with a plain server fails")

	closed := NewSMTPSender(SMTPConfig{Host: "127.0.0.1", Port: freePort(t), From: "chat@example.com"}, clock.Fixed{})
	err = closed.Send(t.Context(), message)
	assert.ErrorContains(t, err, "connecting to mail server")
}

func TestFormatRejectsUnsafeHeaders(t *testing.T) {
	tests := map[string]struct {
		from    string
		message Message
	}{
		"bad sender":           {from: "not an address", message: Message{To: "a@example.com"}},
		"named recipient":      {from: "chat@example.com", message: Message{To: "Eve <eve@example.com>"}},
		"two recipients":       {from: "chat@example.com", message: Message{To: "a@example.com, b@example.com"}},
		"newline in recipient": {from: "chat@example.com", message: Message{To: "a@example.com\r\nBcc: b@example.com"}},
		"newline in subject":   {from: "chat@example.com", message: Message{To: "a@example.com", Subject: "Hi\r\nBcc: b@example.com"}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := format(test.from, test.message, sentAt)

			assert.Error(t, err)
		})
	}
}

func TestSendRejectsBadMessageBeforeConnecting(t *testing.T) {
	unused := NewSMTPSender(SMTPConfig{Host: "127.0.0.1", Port: 1, From: "bad"}, clock.Fixed{})

	err := unused.Send(t.Context(), Message{To: "a@example.com"})

	assert.ErrorContains(t, err, "sender address")
}

func TestDisabled(t *testing.T) {
	assert.ErrorIs(t, Disabled{}.Send(context.Background(), Message{}), ErrNotConfigured)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}
