package mfa

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/mfa/store"
)

// backupCodeCount is spec §4.1: ten single-use backup codes.
const backupCodeCount = 10

// GenerateBackupCodes replaces the user's backup codes with ten new ones and
// returns them. They are shown once; only their hashes are stored.
func (service *Service) GenerateBackupCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	if err := service.store.DeleteBackupCodes(ctx, userID); err != nil {
		return nil, fmt.Errorf("removing old backup codes: %w", err)
	}
	codes := make([]string, 0, backupCodeCount)
	for range backupCodeCount {
		// 10 base32 characters: 50 bits, plenty against guessing when every
		// attempt is rate limited.
		code := rand.Text()[:10]
		hash := hashBackupCode(code)
		if err := service.store.InsertBackupCode(ctx, store.InsertBackupCodeParams{UserID: userID, CodeHash: hash[:]}); err != nil {
			return nil, fmt.Errorf("saving backup code: %w", err)
		}
		codes = append(codes, code[:5]+"-"+code[5:])
	}
	return codes, nil
}

// UseBackupCode reports whether code is one of the user's unused backup
// codes, and uses it up if so.
func (service *Service) UseBackupCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	hash := hashBackupCode(code)
	used, err := service.store.UseBackupCode(ctx, store.UseBackupCodeParams{UserID: userID, CodeHash: hash[:]})
	if err != nil {
		return false, fmt.Errorf("checking backup code: %w", err)
	}
	return used == 1, nil
}

// RemainingBackupCodes returns how many unused backup codes the user has.
func (service *Service) RemainingBackupCodes(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := service.store.CountBackupCodes(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("counting backup codes: %w", err)
	}
	return count, nil
}

// hashBackupCode normalizes (upper case, no dashes or spaces) and hashes.
func hashBackupCode(code string) [sha256.Size]byte {
	normalized := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	return sha256.Sum256([]byte(normalized))
}

// Second-factor method names, as offered on the login screen.
const (
	MethodTOTP        = "totp"
	MethodSecurityKey = "security_key"
	MethodBackupCode  = "backup_code"
)

// Methods returns the second-factor methods the user can log in with.
func (service *Service) Methods(ctx context.Context, userID uuid.UUID) ([]string, error) {
	counts, err := service.store.CountFactorsByKind(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("counting second factors: %w", err)
	}
	var methods []string
	if counts.Totp > 0 {
		methods = append(methods, MethodTOTP)
	}
	if counts.Webauthn > 0 {
		methods = append(methods, MethodSecurityKey)
	}
	if counts.BackupCodes > 0 {
		methods = append(methods, MethodBackupCode)
	}
	return methods, nil
}

// BackupCodesIfFirstFactor creates backup codes when the user has just set
// up their first second factor and has none yet, and returns them (shown
// once). Otherwise it returns nil. Security-key and passkey registration
// call it, matching what ConfirmTOTP does for authenticator apps.
func (service *Service) BackupCodesIfFirstFactor(ctx context.Context, userID uuid.UUID) ([]string, error) {
	factors, err := service.store.CountSecondFactors(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("counting second factors: %w", err)
	}
	remaining, err := service.RemainingBackupCodes(ctx, userID)
	if err != nil {
		return nil, err
	}
	if factors != 1 || remaining > 0 {
		return nil, nil
	}
	return service.GenerateBackupCodes(ctx, userID)
}
