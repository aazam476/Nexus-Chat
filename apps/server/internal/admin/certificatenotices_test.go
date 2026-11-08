package admin

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig"
)

const day = 24 * time.Hour

var certificateExpiry = time.Date(2026, 12, 8, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

func expiring(threshold time.Duration, remaining time.Duration) tlsconfig.Expiry {
	return tlsconfig.Expiry{
		Threshold: threshold, Remaining: remaining,
		Certificate: tlsconfig.Info{Names: []string{"chat.example.com"}, NotAfter: certificateExpiry},
	}
}

func newNotices(settings *fakeSettings, sender *fakeSender) (*CertificateNotices, *bytes.Buffer) {
	var logs bytes.Buffer
	return NewCertificateNotices(settings, sender, "admin@example.com", slog.New(slog.NewTextHandler(&logs, nil))), &logs
}

func TestTheAdminIsEmailedOncePerThreshold(t *testing.T) {
	settings, sender := &fakeSettings{}, &fakeSender{}
	notices, _ := newNotices(settings, sender)

	notices.Notify(t.Context(), expiring(30*day, 29*day+time.Hour))
	notices.Notify(t.Context(), expiring(30*day, 29*day)) // after a restart
	notices.Notify(t.Context(), expiring(7*day, 6*day))

	require.Len(t, sender.sent, 2)
	assert.Equal(t, "admin@example.com", sender.sent[0].To)
	assert.Equal(t, "The HTTPS certificate of Example Chat expires in 29 days", sender.sent[0].Subject)
	assert.Contains(t, sender.sent[0].Text, "(chat.example.com) expires on\n8 December 2026 at 12:00 UTC")
	assert.Equal(t, "The HTTPS certificate of Example Chat expires in 6 days", sender.sent[1].Subject)
}

func TestANewCertificateStartsAgain(t *testing.T) {
	settings, sender := &fakeSettings{}, &fakeSender{}
	notices, _ := newNotices(settings, sender)
	notices.Notify(t.Context(), expiring(7*day, 6*day))

	renewed := expiring(30*day, 20*day)
	renewed.Certificate.NotAfter = certificateExpiry.Add(60 * day)
	notices.Notify(t.Context(), renewed)

	assert.Len(t, sender.sent, 2)
}

func TestFailedNoticesAreLoggedAndTriedAgain(t *testing.T) {
	for _, failOn := range []string{"Load", "InstanceName", "Save"} {
		settings, sender := &fakeSettings{failOn: failOn}, &fakeSender{}
		notices, logs := newNotices(settings, sender)

		notices.Notify(t.Context(), expiring(day, time.Hour))

		assert.Contains(t, logs.String(), "could not email the admin", failOn)
	}

	settings, sender := &fakeSettings{}, &fakeSender{err: errFailure}
	notices, logs := newNotices(settings, sender)
	notices.Notify(t.Context(), expiring(day, time.Hour))
	assert.Contains(t, logs.String(), "database down")
	assert.Nil(t, settings.smtp, "nothing is recorded, so the next start tries again")
}

func TestTimeLeftInWords(t *testing.T) {
	assert.Equal(t, "less than a day (it may already have expired)", timeLeft(0))
	assert.Equal(t, "less than a day", timeLeft(23*time.Hour))
	assert.Equal(t, "1 day", timeLeft(day+time.Hour))
	assert.Equal(t, "30 days", timeLeft(30*day))
}
