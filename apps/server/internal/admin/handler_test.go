package admin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig"
)

type handlerFixture struct {
	handler      *Handler
	certificates *fakeCertificates
	settings     *fakeSettings
	sender       *fakeSender
}

func newHandlerFixture(users fakeUsers) handlerFixture {
	fixture := handlerFixture{certificates: &fakeCertificates{}, settings: &fakeSettings{}, sender: &fakeSender{}}
	fixture.handler = NewHandler(users, fixture.certificates, newMailSettings(fixture.settings, fixture.sender))
	return fixture
}

// everyOperation calls each admin operation with a valid request.
func everyOperation(ctx context.Context, handler *Handler) []any {
	call := func(response any, _ error) any { return response }
	return []any{
		call(handler.GetTlsStatus(ctx, api.GetTlsStatusRequestObject{})),
		call(handler.InstallTlsCertificate(ctx, api.InstallTlsCertificateRequestObject{Body: &api.InstallTlsCertificateRequest{}})),
		call(handler.RemoveTlsCertificate(ctx, api.RemoveTlsCertificateRequestObject{})),
		call(handler.GetSmtpSettings(ctx, api.GetSmtpSettingsRequestObject{})),
		call(handler.UpdateSmtpSettings(ctx, api.UpdateSmtpSettingsRequestObject{Body: &api.UpdateSmtpSettingsRequest{}})),
		call(handler.SendTestEmail(ctx, api.SendTestEmailRequestObject{})),
	}
}

func TestEveryOperationRequiresAnInstanceAdmin(t *testing.T) {
	fixture := newHandlerFixture(fakeUsers{admin: false})

	responses := everyOperation(adminContext(t), fixture.handler)

	want := []any{
		api.GetTlsStatus403JSONResponse{}, api.InstallTlsCertificate403JSONResponse{}, api.RemoveTlsCertificate403JSONResponse{},
		api.GetSmtpSettings403JSONResponse{}, api.UpdateSmtpSettings403JSONResponse{}, api.SendTestEmail403JSONResponse{},
	}
	for index, response := range responses {
		assert.IsType(t, want[index], response)
	}
	assert.Empty(t, fixture.certificates.installed)
	assert.Nil(t, fixture.settings.smtp)
}

func TestEveryOperationReportsLookupFailures(t *testing.T) {
	handler := newHandlerFixture(fakeUsers{err: errFailure}).handler
	ctx := adminContext(t)
	call := func(_ any, err error) error { return err }

	for _, err := range []error{
		call(handler.GetTlsStatus(ctx, api.GetTlsStatusRequestObject{})),
		call(handler.InstallTlsCertificate(ctx, api.InstallTlsCertificateRequestObject{Body: &api.InstallTlsCertificateRequest{}})),
		call(handler.RemoveTlsCertificate(ctx, api.RemoveTlsCertificateRequestObject{})),
		call(handler.GetSmtpSettings(ctx, api.GetSmtpSettingsRequestObject{})),
		call(handler.UpdateSmtpSettings(ctx, api.UpdateSmtpSettingsRequestObject{Body: &api.UpdateSmtpSettingsRequest{}})),
		call(handler.SendTestEmail(ctx, api.SendTestEmailRequestObject{})),
	} {
		assert.ErrorIs(t, err, errFailure)
	}
}

func TestTlsLifecycle(t *testing.T) {
	fixture := newHandlerFixture(fakeUsers{admin: true})
	ctx := adminContext(t)

	status, err := fixture.handler.GetTlsStatus(ctx, api.GetTlsStatusRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, api.GetTlsStatus200JSONResponse{Active: false}, status)

	body := &api.InstallTlsCertificateRequest{Certificate: "CERT", PrivateKey: "KEY"}
	installed, err := fixture.handler.InstallTlsCertificate(ctx, api.InstallTlsCertificateRequestObject{Body: body})
	require.NoError(t, err)
	names, issuer, expires := []string{"chat.example.com"}, "Test CA", time.Time{}
	assert.Equal(t, api.InstallTlsCertificate200JSONResponse{Active: true, Names: &names, Issuer: &issuer, ExpiresAt: &expires}, installed)
	assert.Equal(t, []string{"CERT", "KEY"}, fixture.certificates.installed)

	removed, err := fixture.handler.RemoveTlsCertificate(ctx, api.RemoveTlsCertificateRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, api.RemoveTlsCertificate204Response{}, removed)
	assert.False(t, fixture.certificates.status.Active)
}

func TestInstallTlsCertificateErrors(t *testing.T) {
	fixture := newHandlerFixture(fakeUsers{admin: true})
	fixture.certificates.installErr = &tlsconfig.RejectedError{Err: tlsconfig.ErrIncomplete}
	request := api.InstallTlsCertificateRequestObject{Body: &api.InstallTlsCertificateRequest{}}

	response, err := fixture.handler.InstallTlsCertificate(adminContext(t), request)

	require.NoError(t, err)
	assert.Equal(t, api.InstallTlsCertificate422JSONResponse{Code: "certificate_rejected", Message: tlsconfig.ErrIncomplete.Error()}, response)

	fixture.certificates.installErr = errFailure
	_, err = fixture.handler.InstallTlsCertificate(adminContext(t), request)
	assert.ErrorIs(t, err, errFailure)
}

func TestRemoveTlsCertificateFailure(t *testing.T) {
	fixture := newHandlerFixture(fakeUsers{admin: true})
	fixture.certificates.removeErr = errFailure

	_, err := fixture.handler.RemoveTlsCertificate(adminContext(t), api.RemoveTlsCertificateRequestObject{})

	assert.ErrorIs(t, err, errFailure)
}

func TestSmtpSettingsRoundTripWithoutThePassword(t *testing.T) {
	fixture := newHandlerFixture(fakeUsers{admin: true})
	ctx := adminContext(t)
	username, password := "chat", "secret"
	body := &api.UpdateSmtpSettingsRequest{
		Host: "smtp.example.com", Port: 587, Security: api.SmtpSecurityStarttls,
		Username: &username, Password: &password, From: "chat@example.com",
	}

	updated, err := fixture.handler.UpdateSmtpSettings(ctx, api.UpdateSmtpSettingsRequestObject{Body: body})
	require.NoError(t, err)
	read, err := fixture.handler.GetSmtpSettings(ctx, api.GetSmtpSettingsRequestObject{})
	require.NoError(t, err)

	want := api.SmtpSettings{
		Configured: true, Host: "smtp.example.com", Port: 587, Security: api.SmtpSecurityStarttls,
		Username: "chat", From: "chat@example.com", PasswordSet: true,
	}
	assert.Equal(t, api.UpdateSmtpSettings200JSONResponse(want), updated)
	assert.Equal(t, api.GetSmtpSettings200JSONResponse(want), read)
	assert.Equal(t, "secret", fixture.settings.smtp.Password)
}

func TestUpdateSmtpSettingsErrors(t *testing.T) {
	fixture := newHandlerFixture(fakeUsers{admin: true})
	request := api.UpdateSmtpSettingsRequestObject{Body: &api.UpdateSmtpSettingsRequest{Port: 587, Security: api.SmtpSecurityTls, From: "chat@example.com"}}

	response, err := fixture.handler.UpdateSmtpSettings(adminContext(t), request)

	require.NoError(t, err)
	invalid, ok := response.(api.UpdateSmtpSettings422JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "host", (*invalid.Fields)[0].Field)

	fixture.settings.failOn = "Load"
	_, err = fixture.handler.UpdateSmtpSettings(adminContext(t), request)
	require.ErrorIs(t, err, errFailure)
	_, err = fixture.handler.GetSmtpSettings(adminContext(t), api.GetSmtpSettingsRequestObject{})
	assert.ErrorIs(t, err, errFailure)
}

func TestSendTestEmailResponses(t *testing.T) {
	tests := []struct {
		name    string
		sendErr error
		want    any
	}{
		{"delivered", nil, api.SendTestEmail204Response{}},
		{"not configured", mail.ErrNotConfigured, api.SendTestEmail422JSONResponse{Code: "smtp_not_configured", Message: mail.ErrNotConfigured.Error()}},
		{"refused", &mail.DeliveryError{Err: errFailure}, api.SendTestEmail502JSONResponse{Code: "mail_delivery_failed", Message: errFailure.Error()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHandlerFixture(fakeUsers{admin: true})
			fixture.sender.err = test.sendErr

			response, err := fixture.handler.SendTestEmail(adminContext(t), api.SendTestEmailRequestObject{})

			require.NoError(t, err)
			assert.Equal(t, test.want, response)
		})
	}
}

func TestSendTestEmailGoesToTheCallingAdmin(t *testing.T) {
	fixture := newHandlerFixture(fakeUsers{admin: true})
	ctx := adminContext(t)

	for range 3 {
		_, err := fixture.handler.SendTestEmail(ctx, api.SendTestEmailRequestObject{})
		require.NoError(t, err)
	}
	limited, err := fixture.handler.SendTestEmail(ctx, api.SendTestEmailRequestObject{})

	require.NoError(t, err)
	assert.IsType(t, api.SendTestEmail429JSONResponse{}, limited)
	assert.Equal(t, "ada@example.com", fixture.sender.sent[0].To)
	assert.Contains(t, fixture.sender.sent[0].Text, "Hi Ada,")
}

func TestSendTestEmailReportsUnexpectedFailures(t *testing.T) {
	fixture := newHandlerFixture(fakeUsers{admin: true})
	fixture.sender.err = errFailure

	_, err := fixture.handler.SendTestEmail(adminContext(t), api.SendTestEmailRequestObject{})

	assert.ErrorIs(t, err, errFailure)
}
