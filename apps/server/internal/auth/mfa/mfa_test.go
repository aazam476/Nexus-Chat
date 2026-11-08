package mfa

import (
	"context"
	"encoding/base32"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type movableClock struct{ now time.Time }

func (clock *movableClock) Now() time.Time { return clock.now }

var (
	start = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	alice = uuid.MustParse("01900000-0000-7000-8000-00000000000a")
	key   = []byte("0123456789abcdef0123456789abcdef")
)

func newService(t *testing.T) (*Service, *fakeStore, *movableClock) {
	t.Helper()
	fake := newFakeStore()
	clock := &movableClock{now: start}
	service, err := NewService(fake, key, clock, "Example Chat")
	require.NoError(t, err)
	return service, fake, clock
}

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(secret, at, totp.ValidateOpts{Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	require.NoError(t, err)
	return code
}

func enrolled(t *testing.T, service *Service, clock *movableClock) Enrollment {
	t.Helper()
	enrollment, err := service.EnrollTOTP(t.Context(), alice, "alice@example.com", "Phone")
	require.NoError(t, err)
	_, err = service.ConfirmTOTP(t.Context(), alice, enrollment.ID, codeAt(t, enrollment.Secret, clock.now))
	require.NoError(t, err)
	return enrollment
}

func TestEnrollStoresTheSecretEncrypted(t *testing.T) {
	service, fake, _ := newService(t)

	enrollment, err := service.EnrollTOTP(t.Context(), alice, "alice@example.com", "Phone")

	require.NoError(t, err)
	_, decodeErr := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enrollment.Secret)
	require.NoError(t, decodeErr)
	assert.Contains(t, enrollment.URL, "otpauth://totp/Example%20Chat:alice@example.com")
	stored := fake.credentials[enrollment.ID].EncryptedSecret
	assert.NotContains(t, string(stored), enrollment.Secret)
	assert.Nil(t, fake.credentials[enrollment.ID].ConfirmedAt)
}

func TestFirstConfirmationCreatesBackupCodes(t *testing.T) {
	service, fake, clock := newService(t)
	first, err := service.EnrollTOTP(t.Context(), alice, "alice@example.com", "Phone")
	require.NoError(t, err)

	codes, err := service.ConfirmTOTP(t.Context(), alice, first.ID, codeAt(t, first.Secret, clock.now))

	require.NoError(t, err)
	assert.Len(t, codes, 10)
	assert.Regexp(t, `^[A-Z2-7]{5}-[A-Z2-7]{5}$`, codes[0])
	hasSecond, err := service.HasSecondFactor(t.Context(), alice)
	require.NoError(t, err)
	assert.True(t, hasSecond)

	second, err := service.EnrollTOTP(t.Context(), alice, "alice@example.com", "Tablet")
	require.NoError(t, err)
	codes, err = service.ConfirmTOTP(t.Context(), alice, second.ID, codeAt(t, second.Secret, clock.now))
	require.NoError(t, err)
	assert.Nil(t, codes, "backup codes are created only with the first factor")
	assert.Len(t, fake.codes, 10)
}

func TestConfirmRejectsWrongOrUnknown(t *testing.T) {
	service, _, _ := newService(t)
	enrollment, err := service.EnrollTOTP(t.Context(), alice, "a", "Phone")
	require.NoError(t, err)

	_, err = service.ConfirmTOTP(t.Context(), alice, enrollment.ID, "000000")
	require.ErrorIs(t, err, ErrWrongCode)
	_, err = service.ConfirmTOTP(t.Context(), uuid.New(), enrollment.ID, "000000")
	require.ErrorIs(t, err, ErrEnrollmentNotFound, "another user's enrollment")
	_, err = service.ConfirmTOTP(t.Context(), alice, uuid.New(), "000000")
	assert.ErrorIs(t, err, ErrEnrollmentNotFound)
}

func TestConfirmingTwiceIsNotAllowed(t *testing.T) {
	service, _, clock := newService(t)
	enrollment := enrolled(t, service, clock)
	clock.now = clock.now.Add(time.Minute)

	_, err := service.ConfirmTOTP(t.Context(), alice, enrollment.ID, codeAt(t, enrollment.Secret, clock.now))

	assert.ErrorIs(t, err, ErrEnrollmentNotFound)
}

func TestVerifyAcceptsDriftButNotReplays(t *testing.T) {
	service, _, clock := newService(t)
	enrollment := enrolled(t, service, clock)
	clock.now = start.Add(5 * time.Minute)

	ok, err := service.VerifyTOTP(t.Context(), alice, codeAt(t, enrollment.Secret, clock.now.Add(-30*time.Second)))
	require.NoError(t, err)
	assert.True(t, ok, "a code from the previous step is still accepted")

	ok, err = service.VerifyTOTP(t.Context(), alice, codeAt(t, enrollment.Secret, clock.now.Add(-30*time.Second)))
	require.NoError(t, err)
	assert.False(t, ok, "the same code cannot be used twice")

	ok, err = service.VerifyTOTP(t.Context(), alice, codeAt(t, enrollment.Secret, clock.now.Add(-2*time.Minute)))
	require.NoError(t, err)
	assert.False(t, ok, "codes older than one step are rejected")

	ok, err = service.VerifyTOTP(t.Context(), alice, codeAt(t, enrollment.Secret, clock.now))
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestVerifyWithoutAuthenticators(t *testing.T) {
	service, _, _ := newService(t)

	ok, err := service.VerifyTOTP(t.Context(), alice, "123456")

	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCorruptSecretIsAnError(t *testing.T) {
	service, fake, clock := newService(t)
	enrollment := enrolled(t, service, clock)
	credential := fake.credentials[enrollment.ID]
	credential.EncryptedSecret = []byte("short")
	fake.credentials[enrollment.ID] = credential

	_, err := service.VerifyTOTP(t.Context(), alice, "123456")
	require.ErrorContains(t, err, "too short")

	credential.EncryptedSecret = append(make([]byte, 12), []byte("not a valid ciphertext")...)
	fake.credentials[enrollment.ID] = credential
	_, err = service.VerifyTOTP(t.Context(), alice, "123456")
	assert.ErrorContains(t, err, "decrypting")
}

func TestSecretIsBoundToItsUser(t *testing.T) {
	box, err := newSealer(key)
	require.NoError(t, err)
	sealed := box.seal(alice, []byte("JBSWY3DPEHPK3PXP"))

	_, err = box.open(uuid.New(), sealed)

	assert.Error(t, err)
}

func TestRemoveTOTP(t *testing.T) {
	service, _, clock := newService(t)
	enrollment := enrolled(t, service, clock)

	require.ErrorIs(t, service.RemoveTOTP(t.Context(), uuid.New(), enrollment.ID), ErrNotFound)
	require.NoError(t, service.RemoveTOTP(t.Context(), alice, enrollment.ID))
	hasSecond, err := service.HasSecondFactor(t.Context(), alice)
	require.NoError(t, err)
	assert.False(t, hasSecond)
}

func TestBackupCodesWorkOnceAndIgnoreFormatting(t *testing.T) {
	service, _, _ := newService(t)
	codes, err := service.GenerateBackupCodes(t.Context(), alice)
	require.NoError(t, err)

	used, err := service.UseBackupCode(t.Context(), alice, strings.ToLower(strings.ReplaceAll(codes[0], "-", " ")))
	require.NoError(t, err)
	assert.True(t, used)
	used, err = service.UseBackupCode(t.Context(), alice, codes[0])
	require.NoError(t, err)
	assert.False(t, used)
	remaining, err := service.RemainingBackupCodes(t.Context(), alice)
	require.NoError(t, err)
	assert.Equal(t, int64(9), remaining)

	regenerated, err := service.GenerateBackupCodes(t.Context(), alice)
	require.NoError(t, err)
	used, err = service.UseBackupCode(t.Context(), alice, codes[1])
	require.NoError(t, err)
	assert.False(t, used, "old codes stop working when new ones are made")
	assert.NotEqual(t, codes, regenerated)
}

func TestNewServiceRejectsBadKey(t *testing.T) {
	_, err := NewService(newFakeStore(), []byte("short"), &movableClock{}, "x")

	assert.Error(t, err)
}

func TestStoreFailures(t *testing.T) {
	failure := errors.New("database down")
	service, fake, _ := newService(t)
	fake.fail = failure

	_, err := service.EnrollTOTP(t.Context(), alice, "a", "Phone")
	require.ErrorIs(t, err, failure)
	_, err = service.VerifyTOTP(t.Context(), alice, "123456")
	require.ErrorIs(t, err, failure)
	_, err = service.HasSecondFactor(t.Context(), alice)
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, service.RemoveTOTP(t.Context(), alice, uuid.New()), failure)
	_, err = service.GenerateBackupCodes(t.Context(), alice)
	require.ErrorIs(t, err, failure)
	_, err = service.UseBackupCode(t.Context(), alice, "x")
	require.ErrorIs(t, err, failure)
	_, err = service.RemainingBackupCodes(t.Context(), alice)
	assert.ErrorIs(t, err, failure)
}

func TestIndividualStoreFailures(t *testing.T) {
	failure := errors.New("database down")
	steps := map[string]func(*Service, *movableClock) error{
		"InsertTOTP": func(service *Service, _ *movableClock) error {
			_, err := service.EnrollTOTP(context.Background(), alice, "a", "Phone")
			return err
		},
		"GetTOTP":            confirmStep,
		"CountSecondFactors": confirmStep,
		"ConfirmTOTP":        confirmStep,
		"UseTOTPStep":        confirmStep,
		"InsertBackupCode":   confirmStep,
	}
	for method, step := range steps {
		t.Run(method, func(t *testing.T) {
			service, fake, clock := newService(t)
			fake.fail, fake.failOn = failure, method

			assert.ErrorIs(t, step(service, clock), failure)
		})
	}
}

// confirmStep enrolls (with failures switched off) and then confirms.
func confirmStep(service *Service, clock *movableClock) error {
	fake := service.store.(*fakeStore)
	failure, failOn := fake.fail, fake.failOn
	fake.fail = nil
	enrollment, err := service.EnrollTOTP(context.Background(), alice, "a", "Phone")
	if err != nil {
		return err
	}
	fake.fail, fake.failOn = failure, failOn
	code, err := totp.GenerateCodeCustom(enrollment.Secret, clock.now, totp.ValidateOpts{Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		return err
	}
	_, err = service.ConfirmTOTP(context.Background(), alice, enrollment.ID, code)
	return err
}

func TestEnrollNeedsAnAccountName(t *testing.T) {
	service, _, _ := newService(t)

	_, err := service.EnrollTOTP(t.Context(), alice, "", "Phone")

	assert.ErrorContains(t, err, "creating TOTP secret")
}

func TestListTOTP(t *testing.T) {
	service, fake, clock := newService(t)
	enrollment := enrolled(t, service, clock)
	_, err := service.EnrollTOTP(t.Context(), alice, "a", "Unconfirmed")
	require.NoError(t, err)

	listed, err := service.ListTOTP(t.Context(), alice)
	require.NoError(t, err)
	require.Len(t, listed, 1, "unconfirmed setups are not listed")
	assert.Equal(t, enrollment.ID, listed[0].ID)
	assert.Equal(t, "Phone", listed[0].Name)

	fake.fail = errors.New("down")
	_, err = service.ListTOTP(t.Context(), alice)
	assert.Error(t, err)
}
