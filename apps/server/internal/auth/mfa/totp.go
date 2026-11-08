package mfa

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/mfa/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// TOTP parameters: the values every authenticator app supports
// (RFC 6238 §4 defaults, as used by Google Authenticator and others).
const (
	totpPeriod = 30 * time.Second
	// totpSkew accepts a code from one step before or after now, for phones
	// whose clocks drift a little.
	totpSkew = 1
)

// Errors returned by the TOTP methods.
var (
	ErrEnrollmentNotFound = errors.New("no pending authenticator setup with this ID; start again")
	ErrWrongCode          = errors.New("the code is not correct; check the time on your phone and try again")
	ErrNotFound           = errors.New("no authenticator with this ID")
)

type mfaStore interface {
	InsertTOTP(ctx context.Context, arg store.InsertTOTPParams) error
	GetTOTP(ctx context.Context, arg store.GetTOTPParams) (store.TotpCredential, error)
	ConfirmTOTP(ctx context.Context, arg store.ConfirmTOTPParams) (int64, error)
	ListConfirmedTOTP(ctx context.Context, userID uuid.UUID) ([]store.TotpCredential, error)
	UseTOTPStep(ctx context.Context, arg store.UseTOTPStepParams) (int64, error)
	DeleteTOTP(ctx context.Context, arg store.DeleteTOTPParams) (int64, error)
	DeleteUnconfirmedTOTP(ctx context.Context, userID uuid.UUID) error
	CountSecondFactors(ctx context.Context, userID uuid.UUID) (int64, error)
	DeleteBackupCodes(ctx context.Context, userID uuid.UUID) error
	InsertBackupCode(ctx context.Context, arg store.InsertBackupCodeParams) error
	UseBackupCode(ctx context.Context, arg store.UseBackupCodeParams) (int64, error)
	CountBackupCodes(ctx context.Context, userID uuid.UUID) (int64, error)
}

// Service manages a user's TOTP authenticators and backup codes.
type Service struct {
	store  mfaStore
	sealer sealer
	clock  clock.Clock
	issuer string
}

// NewService returns a Service. key encrypts stored TOTP secrets; issuer is
// the name authenticator apps show next to the code, such as the instance
// name.
func NewService(mfa mfaStore, key []byte, clock clock.Clock, issuer string) (*Service, error) {
	box, err := newSealer(key)
	if err != nil {
		return nil, err
	}
	return &Service{store: mfa, sealer: box, clock: clock, issuer: issuer}, nil
}

// Enrollment is a TOTP app being set up: the user scans URL as a QR code
// (or types Secret), then confirms with a code.
type Enrollment struct {
	ID     uuid.UUID
	Secret string
	URL    string
}

// EnrollTOTP starts adding an authenticator app named name.
func (service *Service) EnrollTOTP(ctx context.Context, userID uuid.UUID, accountName string, name string) (Enrollment, error) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: service.issuer, AccountName: accountName})
	if err != nil {
		return Enrollment{}, fmt.Errorf("creating TOTP secret: %w", err)
	}
	if err := service.store.DeleteUnconfirmedTOTP(ctx, userID); err != nil {
		return Enrollment{}, fmt.Errorf("removing abandoned setups: %w", err)
	}
	id := uuid.Must(uuid.NewV7())
	err = service.store.InsertTOTP(ctx, store.InsertTOTPParams{
		ID: id, UserID: userID, Name: name, EncryptedSecret: service.sealer.seal(userID, []byte(key.Secret())),
		CreatedAt: service.clock.Now(),
	})
	if err != nil {
		return Enrollment{}, fmt.Errorf("saving TOTP setup: %w", err)
	}
	return Enrollment{ID: id, Secret: key.Secret(), URL: key.URL()}, nil
}

// ConfirmTOTP finishes enrollment once the user enters a correct code. If
// this is the user's first second factor, it also creates their backup
// codes and returns them (they are shown once).
func (service *Service) ConfirmTOTP(ctx context.Context, userID uuid.UUID, enrollmentID uuid.UUID, code string) ([]string, error) {
	credential, err := service.store.GetTOTP(ctx, store.GetTOTPParams{ID: enrollmentID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && credential.ConfirmedAt != nil) {
		return nil, ErrEnrollmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading TOTP setup: %w", err)
	}
	if err := service.checkCode(ctx, credential, code); err != nil {
		return nil, err
	}
	existing, err := service.store.CountSecondFactors(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("counting second factors: %w", err)
	}
	if _, err := service.store.ConfirmTOTP(ctx, store.ConfirmTOTPParams{ID: enrollmentID, UserID: userID, ConfirmedAt: ptr(service.clock.Now())}); err != nil {
		return nil, fmt.Errorf("confirming TOTP: %w", err)
	}
	if existing > 0 {
		return nil, nil
	}
	return service.GenerateBackupCodes(ctx, userID)
}

// VerifyTOTP reports whether code is valid for any of the user's confirmed
// authenticators. Each code works only once.
func (service *Service) VerifyTOTP(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	credentials, err := service.store.ListConfirmedTOTP(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("loading authenticators: %w", err)
	}
	for _, credential := range credentials {
		err := service.checkCode(ctx, credential, code)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, ErrWrongCode) {
			return false, err
		}
	}
	return false, nil
}

// HasSecondFactor reports whether login needs a second step for the user.
func (service *Service) HasSecondFactor(ctx context.Context, userID uuid.UUID) (bool, error) {
	count, err := service.store.CountSecondFactors(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("counting second factors: %w", err)
	}
	return count > 0, nil
}

// RemoveTOTP deletes one of the user's authenticators.
func (service *Service) RemoveTOTP(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	deleted, err := service.store.DeleteTOTP(ctx, store.DeleteTOTPParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("removing authenticator: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

// checkCode accepts code if it matches the current step (or one either
// side) and that step is later than the last one used.
func (service *Service) checkCode(ctx context.Context, credential store.TotpCredential, code string) error {
	secret, err := service.sealer.open(credential.UserID, credential.EncryptedSecret)
	if err != nil {
		return err
	}
	step, matched := matchStep(string(secret), code, service.clock.Now())
	if !matched {
		return ErrWrongCode
	}
	used, err := service.store.UseTOTPStep(ctx, store.UseTOTPStepParams{ID: credential.ID, LastUsedStep: step})
	if err != nil {
		return fmt.Errorf("recording TOTP use: %w", err)
	}
	if used == 0 {
		return ErrWrongCode // replayed code
	}
	return nil
}

// matchStep returns the time step whose code equals code, checking the
// current step and totpSkew steps either side, in constant time per code.
func matchStep(secret string, code string, now time.Time) (int64, bool) {
	current := now.Unix() / int64(totpPeriod.Seconds())
	options := totp.ValidateOpts{Period: uint(totpPeriod.Seconds()), Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1}
	for offset := int64(-totpSkew); offset <= totpSkew; offset++ {
		step := current + offset
		expected, err := totp.GenerateCodeCustom(secret, time.Unix(step*int64(totpPeriod.Seconds()), 0), options)
		if err == nil && subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

func ptr[T any](value T) *T {
	return &value
}
