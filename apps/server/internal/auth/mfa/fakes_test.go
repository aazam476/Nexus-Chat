package mfa

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/mfa/store"
)

// fakeStore keeps TOTP credentials and backup codes in memory.
type fakeStore struct {
	credentials map[uuid.UUID]store.TotpCredential
	codes       map[string]bool // userID + hash
	fail        error
	// failOn names the one method that returns fail; "" means every method.
	failOn string
	// webauthn is how many security keys the user has (kept elsewhere).
	webauthn int64
}

// failure returns the configured error if method should fail.
func (fake *fakeStore) failure(method string) error {
	if fake.failOn == "" || fake.failOn == method {
		return fake.fail
	}
	return nil
}

func newFakeStore() *fakeStore {
	return &fakeStore{credentials: map[uuid.UUID]store.TotpCredential{}, codes: map[string]bool{}}
}

func (fake *fakeStore) InsertTOTP(_ context.Context, arg store.InsertTOTPParams) error {
	fake.credentials[arg.ID] = store.TotpCredential{ID: arg.ID, UserID: arg.UserID, Name: arg.Name, EncryptedSecret: arg.EncryptedSecret, CreatedAt: arg.CreatedAt}
	return fake.failure("InsertTOTP")
}

func (fake *fakeStore) GetTOTP(_ context.Context, arg store.GetTOTPParams) (store.TotpCredential, error) {
	credential, ok := fake.credentials[arg.ID]
	if !ok || credential.UserID != arg.UserID {
		return store.TotpCredential{}, pgx.ErrNoRows
	}
	return credential, fake.failure("GetTOTP")
}

func (fake *fakeStore) ConfirmTOTP(_ context.Context, arg store.ConfirmTOTPParams) (int64, error) {
	credential := fake.credentials[arg.ID]
	credential.ConfirmedAt = arg.ConfirmedAt
	fake.credentials[arg.ID] = credential
	return 1, fake.failure("ConfirmTOTP")
}

func (fake *fakeStore) ListConfirmedTOTP(_ context.Context, userID uuid.UUID) ([]store.TotpCredential, error) {
	var result []store.TotpCredential
	for _, credential := range fake.credentials {
		if credential.UserID == userID && credential.ConfirmedAt != nil {
			result = append(result, credential)
		}
	}
	return result, fake.failure("ListConfirmedTOTP")
}

func (fake *fakeStore) UseTOTPStep(_ context.Context, arg store.UseTOTPStepParams) (int64, error) {
	credential := fake.credentials[arg.ID]
	if credential.LastUsedStep >= arg.LastUsedStep {
		return 0, fake.failure("UseTOTPStep")
	}
	credential.LastUsedStep = arg.LastUsedStep
	fake.credentials[arg.ID] = credential
	return 1, fake.failure("UseTOTPStep")
}

func (fake *fakeStore) DeleteTOTP(_ context.Context, arg store.DeleteTOTPParams) (int64, error) {
	credential, ok := fake.credentials[arg.ID]
	if !ok || credential.UserID != arg.UserID {
		return 0, fake.failure("DeleteTOTP")
	}
	delete(fake.credentials, arg.ID)
	return 1, fake.failure("DeleteTOTP")
}

func (fake *fakeStore) DeleteUnconfirmedTOTP(_ context.Context, userID uuid.UUID) error {
	for id, credential := range fake.credentials {
		if credential.UserID == userID && credential.ConfirmedAt == nil {
			delete(fake.credentials, id)
		}
	}
	return fake.failure("DeleteUnconfirmedTOTP")
}

func (fake *fakeStore) CountSecondFactors(_ context.Context, userID uuid.UUID) (int64, error) {
	if err := fake.failure("CountSecondFactors"); err != nil {
		return 0, err
	}
	var count int64
	for _, credential := range fake.credentials {
		if credential.UserID == userID && credential.ConfirmedAt != nil {
			count++
		}
	}
	return count, nil
}

func (fake *fakeStore) DeleteBackupCodes(_ context.Context, userID uuid.UUID) error {
	for key := range fake.codes {
		if strings.HasPrefix(key, userID.String()) {
			delete(fake.codes, key)
		}
	}
	return fake.failure("DeleteBackupCodes")
}

func (fake *fakeStore) InsertBackupCode(_ context.Context, arg store.InsertBackupCodeParams) error {
	fake.codes[arg.UserID.String()+string(arg.CodeHash)] = true
	return fake.failure("InsertBackupCode")
}

func (fake *fakeStore) UseBackupCode(_ context.Context, arg store.UseBackupCodeParams) (int64, error) {
	key := arg.UserID.String() + string(arg.CodeHash)
	if !fake.codes[key] {
		return 0, fake.failure("UseBackupCode")
	}
	delete(fake.codes, key)
	return 1, fake.failure("UseBackupCode")
}

func (fake *fakeStore) CountBackupCodes(_ context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for key := range fake.codes {
		if strings.HasPrefix(key, userID.String()) {
			count++
		}
	}
	return count, fake.failure("CountBackupCodes")
}

func (fake *fakeStore) CountFactorsByKind(ctx context.Context, userID uuid.UUID) (store.CountFactorsByKindRow, error) {
	if err := fake.failure("CountFactorsByKind"); err != nil {
		return store.CountFactorsByKindRow{}, err
	}
	totp, _ := fake.CountSecondFactors(ctx, userID)
	codes, _ := fake.CountBackupCodes(ctx, userID)
	return store.CountFactorsByKindRow{Totp: totp, Webauthn: fake.webauthn, BackupCodes: codes}, nil
}
