package login

import (
	"context"
	"errors"
	"fmt"

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

// CompleteMFA checks the second factor and starts the session.
func (service *Service) CompleteMFA(ctx context.Context, request MFARequest) (Result, error) {
	if !service.dependencies.Limiter.Allow(request.ClientKey) {
		return Result{}, ErrRateLimited
	}
	userID, err := service.dependencies.Challenges.Redeem(ctx, tokens.MFALogin, request.Challenge)
	if errors.Is(err, tokens.ErrInvalidToken) {
		return Result{}, ErrInvalidSecondFactor
	}
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
	user, err := service.dependencies.Accounts.GetUserByID(ctx, userID)
	if err != nil {
		return Result{}, fmt.Errorf("loading account: %w", err)
	}
	return service.startSession(ctx, user, request.Device)
}
