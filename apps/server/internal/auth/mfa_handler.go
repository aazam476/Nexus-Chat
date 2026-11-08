package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/mfa"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
)

// secondFactorManager is satisfied by *mfa.Service.
type secondFactorManager interface {
	EnrollTOTP(ctx context.Context, userID uuid.UUID, accountName string, name string) (mfa.Enrollment, error)
	ConfirmTOTP(ctx context.Context, userID uuid.UUID, enrollmentID uuid.UUID, code string) ([]string, error)
	ListTOTP(ctx context.Context, userID uuid.UUID) ([]mfa.Authenticator, error)
	RemoveTOTP(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	GenerateBackupCodes(ctx context.Context, userID uuid.UUID) ([]string, error)
	RemainingBackupCodes(ctx context.Context, userID uuid.UUID) (int64, error)
}

// CompleteLoginMfa answers POST /api/v1/auth/login/mfa.
func (handler *Handler) CompleteLoginMfa(ctx context.Context, request api.CompleteLoginMfaRequestObject) (api.CompleteLoginMfaResponseObject, error) {
	result, err := handler.dependencies.Logins.CompleteMFA(ctx, login.MFARequest{
		Challenge: request.Body.ChallengeToken, Method: string(request.Body.Method), Code: request.Body.Code,
		Device: session.Device{Name: apiresponse.DeviceName(request.Body.DeviceName)}, ClientKey: clientip.FromContext(ctx).Address.String(),
	})
	switch {
	case errors.Is(err, login.ErrInvalidSecondFactor):
		return api.CompleteLoginMfa401JSONResponse(apiresponse.Error("invalid_second_factor", err)), nil
	case errors.Is(err, login.ErrRateLimited):
		return api.CompleteLoginMfa429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case err != nil:
		return nil, err
	}
	return api.CompleteLoginMfa200JSONResponse{
		Token: result.Token, Session: apiresponse.Session(result.Session, result.Session.ID), User: accounts.CurrentUserResponse(result.User),
	}, nil
}

// GetSecondFactors answers GET /api/v1/auth/mfa.
func (handler *Handler) GetSecondFactors(ctx context.Context, _ api.GetSecondFactorsRequestObject) (api.GetSecondFactorsResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	authenticators, err := handler.dependencies.MFA.ListTOTP(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	remaining, err := handler.dependencies.MFA.RemainingBackupCodes(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	response := api.GetSecondFactors200JSONResponse{Authenticators: make([]api.Authenticator, 0, len(authenticators)), BackupCodesRemaining: remaining}
	for _, authenticator := range authenticators {
		response.Authenticators = append(response.Authenticators, api.Authenticator{Id: authenticator.ID, Name: authenticator.Name, CreatedAt: authenticator.CreatedAt})
	}
	return response, nil
}

// EnrollTotp answers POST /api/v1/auth/mfa/totp.
func (handler *Handler) EnrollTotp(ctx context.Context, request api.EnrollTotpRequestObject) (api.EnrollTotpResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	user, err := handler.dependencies.Users.GetUserByID(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	enrollment, err := handler.dependencies.MFA.EnrollTOTP(ctx, user.ID, user.Username, apiresponse.DeviceName(request.Body.Name))
	if err != nil {
		return nil, err
	}
	return api.EnrollTotp201JSONResponse{EnrollmentId: enrollment.ID, Secret: enrollment.Secret, OtpauthUrl: enrollment.URL}, nil
}

// ConfirmTotp answers POST /api/v1/auth/mfa/totp/{enrollmentId}/confirm.
func (handler *Handler) ConfirmTotp(ctx context.Context, request api.ConfirmTotpRequestObject) (api.ConfirmTotpResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	codes, err := handler.dependencies.MFA.ConfirmTOTP(ctx, principal.UserID, request.EnrollmentId, request.Body.Code)
	switch {
	case errors.Is(err, mfa.ErrWrongCode):
		return api.ConfirmTotp400JSONResponse(apiresponse.Error("invalid_code", err)), nil
	case errors.Is(err, mfa.ErrEnrollmentNotFound):
		return api.ConfirmTotp400JSONResponse(apiresponse.Error("enrollment_not_found", err)), nil
	case err != nil:
		return nil, err
	}
	response := api.ConfirmTotp200JSONResponse{}
	if codes != nil {
		response.BackupCodes = &codes
	}
	return response, nil
}

// RemoveTotp answers DELETE /api/v1/auth/mfa/totp/{authenticatorId}.
func (handler *Handler) RemoveTotp(ctx context.Context, request api.RemoveTotpRequestObject) (api.RemoveTotpResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	err := handler.dependencies.MFA.RemoveTOTP(ctx, principal.UserID, request.AuthenticatorId)
	if errors.Is(err, mfa.ErrNotFound) {
		return api.RemoveTotp404JSONResponse(apiresponse.Error("authenticator_not_found", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.RemoveTotp204Response{}, nil
}

// RegenerateBackupCodes answers POST /api/v1/auth/mfa/backup-codes.
func (handler *Handler) RegenerateBackupCodes(ctx context.Context, _ api.RegenerateBackupCodesRequestObject) (api.RegenerateBackupCodesResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	codes, err := handler.dependencies.MFA.GenerateBackupCodes(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	return api.RegenerateBackupCodes200JSONResponse{Codes: codes}, nil
}
