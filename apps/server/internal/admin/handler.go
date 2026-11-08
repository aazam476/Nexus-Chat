package admin

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig"
)

// Certificates installs and removes the HTTPS certificate.
type Certificates interface {
	Status() tlsconfig.Status
	Install(ctx context.Context, certificatePEM []byte, keyPEM []byte) (tlsconfig.Info, error)
	Remove() error
}

// Handler serves the admin TLS and SMTP endpoints.
type Handler struct {
	users        accounts.UserReader
	certificates Certificates
	mail         *MailSettings
}

// NewHandler returns a Handler. users is used to check that the caller is
// an instance admin.
func NewHandler(users accounts.UserReader, certificates Certificates, mailSettings *MailSettings) *Handler {
	return &Handler{users: users, certificates: certificates, mail: mailSettings}
}

// requireAdmin returns the caller, or the body of a 403 response if they
// are not an instance admin.
func (handler *Handler) requireAdmin(ctx context.Context) (accountsstore.User, *api.ForbiddenJSONResponse, error) {
	admin, err := accounts.RequireInstanceAdmin(ctx, handler.users)
	if errors.Is(err, accounts.ErrForbidden) {
		forbidden := api.ForbiddenJSONResponse(apiresponse.Error("forbidden", err))
		return accountsstore.User{}, &forbidden, nil
	}
	return admin, nil, err
}

// GetTlsStatus answers GET /api/v1/admin/tls.
func (handler *Handler) GetTlsStatus(ctx context.Context, _ api.GetTlsStatusRequestObject) (api.GetTlsStatusResponseObject, error) {
	_, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.GetTlsStatus403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetTlsStatus200JSONResponse(tlsStatus(handler.certificates.Status())), nil
}

// InstallTlsCertificate answers PUT /api/v1/admin/tls.
func (handler *Handler) InstallTlsCertificate(ctx context.Context, request api.InstallTlsCertificateRequestObject) (api.InstallTlsCertificateResponseObject, error) {
	_, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.InstallTlsCertificate403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = handler.certificates.Install(ctx, []byte(request.Body.Certificate), []byte(request.Body.PrivateKey))
	var rejected *tlsconfig.RejectedError
	if errors.As(err, &rejected) {
		return api.InstallTlsCertificate422JSONResponse(apiresponse.Error("certificate_rejected", rejected)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.InstallTlsCertificate200JSONResponse(tlsStatus(handler.certificates.Status())), nil
}

// RemoveTlsCertificate answers DELETE /api/v1/admin/tls.
func (handler *Handler) RemoveTlsCertificate(ctx context.Context, _ api.RemoveTlsCertificateRequestObject) (api.RemoveTlsCertificateResponseObject, error) {
	_, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.RemoveTlsCertificate403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := handler.certificates.Remove(); err != nil {
		return nil, err
	}
	return api.RemoveTlsCertificate204Response{}, nil
}

func tlsStatus(status tlsconfig.Status) api.TlsStatus {
	if !status.Active {
		return api.TlsStatus{Active: false}
	}
	names := append([]string{}, status.Info.Names...)
	return api.TlsStatus{Active: true, Names: &names, Issuer: &status.Info.Issuer, ExpiresAt: &status.Info.NotAfter}
}

// GetSmtpSettings answers GET /api/v1/admin/smtp.
func (handler *Handler) GetSmtpSettings(ctx context.Context, _ api.GetSmtpSettingsRequestObject) (api.GetSmtpSettingsResponseObject, error) {
	_, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.GetSmtpSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	config, _, err := handler.mail.Current(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetSmtpSettings200JSONResponse(smtpSettings(config)), nil
}

// UpdateSmtpSettings answers PUT /api/v1/admin/smtp.
func (handler *Handler) UpdateSmtpSettings(ctx context.Context, request api.UpdateSmtpSettingsRequestObject) (api.UpdateSmtpSettingsResponseObject, error) {
	_, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.UpdateSmtpSettings403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	body := request.Body
	update := SMTPUpdate{
		Host: body.Host, Port: int(body.Port), Security: mail.Security(body.Security),
		Password: body.Password, From: body.From,
	}
	if body.Username != nil {
		update.Username = *body.Username
	}
	config, err := handler.mail.Update(ctx, update)
	var problems *validation.Error
	if errors.As(err, &problems) {
		return api.UpdateSmtpSettings422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateSmtpSettings200JSONResponse(smtpSettings(config)), nil
}

func smtpSettings(config mail.SMTPConfig) api.SmtpSettings {
	return api.SmtpSettings{
		Configured: config.Host != "", Host: config.Host, Port: int32(config.Port), //nolint:gosec // G115: saved ports are at most 65535
		Security: api.SmtpSecurity(config.Security), Username: config.Username, From: config.From,
		PasswordSet: config.Password != "",
	}
}

// SendTestEmail answers POST /api/v1/admin/smtp/test.
func (handler *Handler) SendTestEmail(ctx context.Context, _ api.SendTestEmailRequestObject) (api.SendTestEmailResponseObject, error) {
	admin, forbidden, err := handler.requireAdmin(ctx)
	if forbidden != nil {
		return api.SendTestEmail403JSONResponse{ForbiddenJSONResponse: *forbidden}, nil
	}
	if err != nil {
		return nil, err
	}
	err = handler.mail.SendTest(ctx, admin.ID.String(), admin.Email, admin.DisplayName)
	var delivery *mail.DeliveryError
	switch {
	case err == nil:
		return api.SendTestEmail204Response{}, nil
	case errors.Is(err, ErrTestRateLimited):
		return api.SendTestEmail429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case errors.Is(err, mail.ErrNotConfigured):
		return api.SendTestEmail422JSONResponse(apiresponse.Error("smtp_not_configured", err)), nil
	case errors.As(err, &delivery):
		return api.SendTestEmail502JSONResponse(apiresponse.Error("mail_delivery_failed", delivery)), nil
	}
	return nil, err
}
