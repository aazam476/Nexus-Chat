package mail

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderEveryTemplate(t *testing.T) {
	data := TemplateData{InstanceName: "Example Chat", DisplayName: "Alice", Link: "https://chat.example.com/x#token=abc"}

	for _, name := range []string{TemplateVerifyEmail, TemplateResetPassword} {
		t.Run(name, func(t *testing.T) {
			message, err := Render(name, "alice@example.com", data)

			require.NoError(t, err)
			assert.Equal(t, "alice@example.com", message.To)
			assert.Contains(t, message.Subject, "Example Chat")
			assert.NotContains(t, message.Subject, "\n")
			assert.Contains(t, message.Text, "Hi Alice,")
			assert.Contains(t, message.Text, data.Link)
		})
	}
}

func TestRenderTestEmail(t *testing.T) {
	message, err := Render(TemplateTestEmail, "admin@example.com", TemplateData{InstanceName: "Example Chat", DisplayName: "Ada"})

	require.NoError(t, err)
	assert.Equal(t, "Test email from Example Chat", message.Subject)
	assert.Contains(t, message.Text, "Hi Ada,")
}

func TestRenderCertificateWarning(t *testing.T) {
	message, err := Render(TemplateCertificateExpiring, "admin@example.com", TemplateData{
		InstanceName: "Example Chat", Domain: "chat.example.com", Expires: "8 December 2026 at 12:00 UTC", DaysLeft: "7 days",
	})

	require.NoError(t, err)
	assert.Equal(t, "The HTTPS certificate of Example Chat expires in 7 days", message.Subject)
	assert.Contains(t, message.Text, "(chat.example.com) expires on\n8 December 2026 at 12:00 UTC, in 7 days.")
}

func TestRenderUnknownTemplate(t *testing.T) {
	_, err := Render("missing", "a@example.com", TemplateData{})

	assert.ErrorContains(t, err, "loading email template missing")
}
