package login

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

// Second-factor methods accepted by CompleteMFA.
const (
	MethodTOTP       = "totp"
	MethodBackupCode = "backup_code"
)

// ErrInvalidSecondFactor means the code was wrong or the challenge is
// unknown, used or expired. Either way the user starts again with their
// password: each challenge allows exactly one attempt, which makes
// guessing six-digit codes as slow as guessing passwords.
var ErrInvalidSecondFactor = errors.New("the code is not correct or the login has expired; log in again")

// MFARequest finishes a login that needs a second factor.
type MFARequest struct {
	Challenge string
	Method    string
	Code      string
	Device    session.Device
	ClientKey string
}

// RedeemChallenge uses up an MFA challenge and returns whose login it is.
// Security keys call it when their ceremony starts, so the challenge still
// allows only one attempt.
func (service *Service) RedeemChallenge(ctx context.Context, challenge string, clientKey string) (uuid.UUID, error) {
	if !service.dependencies.Limiter.Allow(clientKey) {
		return uuid.Nil, ErrRateLimited
	}
	userID, err := service.dependencies.Challenges.Redeem(ctx, tokens.MFALogin, challenge)
	if errors.Is(err, tokens.ErrInvalidToken) {
		return uuid.Nil, ErrInvalidSecondFactor
	}
	return userID, err
}

// StartSession logs userID in once every factor has been checked elsewhere
// (a security key, or a passkey alone).
func (service *Service) StartSession(ctx context.Context, userID uuid.UUID, device session.Device) (Result, error) {
	user, err := service.dependencies.Accounts.GetUserByID(ctx, userID)
	if err != nil {
		return Result{}, fmt.Errorf("loading account: %w", err)
	}
	return service.startSession(ctx, user, device)
}

// CompleteMFA checks a TOTP or backup code and starts the session.
func (service *Service) CompleteMFA(ctx context.Context, request MFARequest) (Result, error) {
	userID, err := service.RedeemChallenge(ctx, request.Challenge, request.ClientKey)
	if err != nil {
		return Result{}, err
	}
	var accepted bool
	switch request.Method {
	case MethodTOTP:
		accepted, err = service.dependencies.SecondFactors.VerifyTOTP(ctx, userID, request.Code)
	case MethodBackupCode:
		accepted, err = service.dependencies.SecondFactors.UseBackupCode(ctx, userID, request.Code)
	}
	if err != nil {
		return Result{}, err
	}
	if !accepted {
		return Result{}, ErrInvalidSecondFactor
	}
	return service.StartSession(ctx, userID, request.Device)
}
