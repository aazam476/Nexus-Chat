package auth

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

var errAlreadyVerified = errors.New("this email address is already verified")

// VerifyEmail answers POST /api/v1/auth/verify-email.
func (handler *Handler) VerifyEmail(ctx context.Context, request api.VerifyEmailRequestObject) (api.VerifyEmailResponseObject, error) {
	err := handler.dependencies.Verifier.Verify(ctx, request.Body.Token)
	if errors.Is(err, tokens.ErrInvalidToken) {
		return api.VerifyEmail400JSONResponse(apiresponse.Error("invalid_token", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.VerifyEmail204Response{}, nil
}

// ResendVerificationEmail answers POST /api/v1/auth/verify-email/resend.
func (handler *Handler) ResendVerificationEmail(ctx context.Context, _ api.ResendVerificationEmailRequestObject) (api.ResendVerificationEmailResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	user, err := handler.dependencies.Users.GetUserByID(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	if user.EmailVerifiedAt != nil {
		return api.ResendVerificationEmail409JSONResponse(apiresponse.Error("already_verified", errAlreadyVerified)), nil
	}
	if err := handler.dependencies.Emails.SendVerification(ctx, user); err != nil {
		// Missing SMTP or domain settings are the usual cause; the message
		// says which, so the user can tell their admin.
		return api.ResendVerificationEmail503JSONResponse(apiresponse.Error("email_unavailable", err)), nil
	}
	return api.ResendVerificationEmail202Response{}, nil
}

// RequestPasswordReset answers POST /api/v1/auth/password-reset.
func (handler *Handler) RequestPasswordReset(ctx context.Context, request api.RequestPasswordResetRequestObject) (api.RequestPasswordResetResponseObject, error) {
	// Request's only possible error is ErrRateLimited: every other problem is
	// logged and hidden, so the response never reveals whether the address
	// has an account.
	if err := handler.dependencies.Resets.Request(ctx, request.Body.Email, clientip.FromContext(ctx).Address.String()); err != nil {
		return api.RequestPasswordReset429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	}
	return api.RequestPasswordReset202Response{}, nil
}

// CompletePasswordReset answers POST /api/v1/auth/password-reset/complete.
func (handler *Handler) CompletePasswordReset(ctx context.Context, request api.CompletePasswordResetRequestObject) (api.CompletePasswordResetResponseObject, error) {
	err := handler.dependencies.Resets.Complete(ctx, request.Body.Token, request.Body.Password)
	if errors.Is(err, tokens.ErrInvalidToken) {
		return api.CompletePasswordReset400JSONResponse(apiresponse.Error("invalid_token", err)), nil
	}
	var problems *validation.Error
	if errors.As(err, &problems) {
		return api.CompletePasswordReset422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Validation(problems))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.CompletePasswordReset204Response{}, nil
}
