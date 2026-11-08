package admin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

func validUpdate() SMTPUpdate {
	password := "secret"
	return SMTPUpdate{
		Host: " smtp.example.com ", Port: 587, Security: mail.SecurityStartTLS,
		Username: "chat", Password: &password, From: "chat@example.com",
	}
}

func TestCurrentDefaultsBeforeAnythingIsSaved(t *testing.T) {
	config, configured, err := newMailSettings(&fakeSettings{}, &fakeSender{}).Current(t.Context())

	require.NoError(t, err)
	assert.False(t, configured)
	assert.Equal(t, mail.SMTPConfig{Port: 587, Security: mail.SecurityStartTLS}, config)
}

func TestUpdateSavesAndKeepsPasswordWhenLeftOut(t *testing.T) {
	settings := &fakeSettings{}
	service := newMailSettings(settings, &fakeSender{})

	saved, err := service.Update(t.Context(), validUpdate())
	require.NoError(t, err)
	assert.Equal(t, "smtp.example.com", saved.Host, "whitespace is trimmed")

	update := validUpdate()
	update.Password = nil
	update.Port = 465
	update.Security = mail.SecurityTLS
	saved, err = service.Update(t.Context(), update)
	require.NoError(t, err)
	assert.Equal(t, "secret", saved.Password)

	cleared := ""
	update.Password = &cleared
	saved, err = service.Update(t.Context(), update)
	require.NoError(t, err)
	assert.Empty(t, saved.Password)

	config, configured, err := service.Current(t.Context())
	require.NoError(t, err)
	assert.True(t, configured)
	assert.Equal(t, saved, config)
}

func TestUpdateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		change func(*SMTPUpdate)
		field  string
	}{
		{"no host", func(update *SMTPUpdate) { update.Host = " " }, "host"},
		{"host with spaces", func(update *SMTPUpdate) { update.Host = "smtp example.com" }, "host"},
		{"host with slash", func(update *SMTPUpdate) { update.Host = "smtp.example.com/x" }, "host"},
		{"host too long", func(update *SMTPUpdate) { update.Host = strings.Repeat("a", 254) }, "host"},
		{"port zero", func(update *SMTPUpdate) { update.Port = 0 }, "port"},
		{"port too large", func(update *SMTPUpdate) { update.Port = 65536 }, "port"},
		{"unknown security", func(update *SMTPUpdate) { update.Security = "ssl" }, "security"},
		{"password in the clear", func(update *SMTPUpdate) { update.Security = mail.SecurityNone }, "security"},
		{"username too long", func(update *SMTPUpdate) { update.Username = strings.Repeat("u", 321) }, "username"},
		{"password too long", func(update *SMTPUpdate) {
			long := strings.Repeat("p", 1025)
			update.Password = &long
		}, "password"},
		{"from with a name", func(update *SMTPUpdate) { update.From = "Chat <chat@example.com>" }, "from"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := &fakeSettings{}
			update := validUpdate()
			test.change(&update)

			_, err := newMailSettings(settings, &fakeSender{}).Update(t.Context(), update)

			var problems *validation.Error
			require.ErrorAs(t, err, &problems)
			require.Len(t, problems.Fields, 1)
			assert.Equal(t, test.field, problems.Fields[0].Field)
			assert.Nil(t, settings.smtp, "nothing is saved")
		})
	}
}

func TestUpdateAllowsUnencryptedRelays(t *testing.T) {
	update := validUpdate()
	update.Security = mail.SecurityNone
	update.Host = "localhost"
	_, err := newMailSettings(&fakeSettings{}, &fakeSender{}).Update(t.Context(), update)
	require.NoError(t, err, "a password may go to a relay on this machine")

	for _, host := range []string{"127.0.0.1", "::1", "relay.internal"} {
		update.Host = host
		if host == "relay.internal" {
			update.Password = nil
			update.Username = ""
		}
		_, err := newMailSettings(&fakeSettings{}, &fakeSender{}).Update(t.Context(), update)
		assert.NoError(t, err, host)
	}
}

func TestUpdateReportsStoreFailures(t *testing.T) {
	for _, method := range []string{"Load", "Save"} {
		_, err := newMailSettings(&fakeSettings{failOn: method}, &fakeSender{}).Update(t.Context(), validUpdate())
		assert.ErrorIs(t, err, errFailure, method)
	}
}

func TestSendTest(t *testing.T) {
	sender := &fakeSender{}
	service := newMailSettings(&fakeSettings{}, sender)

	require.NoError(t, service.SendTest(t.Context(), "admin", "ada@example.com", "Ada"))

	require.Len(t, sender.sent, 1)
	assert.Equal(t, "ada@example.com", sender.sent[0].To)
	assert.Equal(t, "Test email from Example Chat", sender.sent[0].Subject)
}

func TestSendTestIsRateLimitedPerAdmin(t *testing.T) {
	service := newMailSettings(&fakeSettings{}, &fakeSender{})
	for range 3 {
		require.NoError(t, service.SendTest(t.Context(), "admin", "ada@example.com", "Ada"))
	}

	require.ErrorIs(t, service.SendTest(t.Context(), "admin", "ada@example.com", "Ada"), ErrTestRateLimited)
	assert.NoError(t, service.SendTest(t.Context(), "other", "bo@example.com", "Bo"))
}

func TestSendTestReportsFailures(t *testing.T) {
	err := newMailSettings(&fakeSettings{failOn: "InstanceName"}, &fakeSender{}).SendTest(t.Context(), "a", "ada@example.com", "Ada")
	require.ErrorIs(t, err, errFailure)

	err = newMailSettings(&fakeSettings{}, &fakeSender{err: mail.ErrNotConfigured}).SendTest(t.Context(), "a", "ada@example.com", "Ada")
	assert.ErrorIs(t, err, mail.ErrNotConfigured)
}
