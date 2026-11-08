package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func totpCode(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(secret, at, totp.ValidateOpts{Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	require.NoError(t, err)
	return code
}

func mfaBody(challenge string, method string, code string) string {
	return fmt.Sprintf(`{"challengeToken": %q, "method": %q, "code": %q, "deviceName": "Laptop"}`, challenge, method, code)
}

func TestTOTPLogin(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")

	var enrollment struct {
		EnrollmentID string `json:"enrollmentId"`
		Secret       string
		OtpauthURL   string `json:"otpauthUrl"`
	}
	body := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/auth/mfa/totp", body: `{"name": "Phone"}`}, http.StatusCreated)
	require.NoError(t, json.Unmarshal(body, &enrollment))
	assert.Contains(t, enrollment.OtpauthURL, "otpauth://totp/")

	var confirmation struct{ BackupCodes []string }
	confirm := fmt.Sprintf(`{"code": %q}`, totpCode(t, enrollment.Secret, time.Now()))
	body = server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/auth/mfa/totp/" + enrollment.EnrollmentID + "/confirm", body: confirm}, http.StatusOK)
	require.NoError(t, json.Unmarshal(body, &confirmation))
	require.Len(t, confirmation.BackupCodes, 10)

	// The password alone now leads to a second-factor challenge.
	var challenge struct{ ChallengeToken string }
	body = server.call(t, http.MethodPost, "/api/v1/auth/login", loginBody("admin", "a long password", "Laptop"), http.StatusAccepted)
	require.NoError(t, json.Unmarshal(body, &challenge))

	// The confirmation used the current 30-second step, so log in with the
	// next step's code (accepted as clock drift; a reused code would not be).
	next := totpCode(t, enrollment.Secret, time.Now().Add(30*time.Second))
	server.call(t, http.MethodPost, "/api/v1/auth/login/mfa", mfaBody(challenge.ChallengeToken, "totp", next), http.StatusOK)

	// A backup code works once, through a fresh challenge.
	body = server.call(t, http.MethodPost, "/api/v1/auth/login", loginBody("admin", "a long password", "Laptop"), http.StatusAccepted)
	require.NoError(t, json.Unmarshal(body, &challenge))
	server.call(t, http.MethodPost, "/api/v1/auth/login/mfa", mfaBody(challenge.ChallengeToken, "backup_code", confirmation.BackupCodes[0]), http.StatusOK)

	listed := server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/auth/mfa", http.StatusOK)
	assert.Contains(t, string(listed), `"backupCodesRemaining":9`)
}

func TestWrongSecondFactorEndsTheChallenge(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	var enrollment struct {
		EnrollmentID string `json:"enrollmentId"`
		Secret       string
	}
	body := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/auth/mfa/totp", body: `{"name": "Phone"}`}, http.StatusCreated)
	require.NoError(t, json.Unmarshal(body, &enrollment))
	server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/auth/mfa/totp/" + enrollment.EnrollmentID + "/confirm",
		body: `{"code": "000000"}`}, http.StatusBadRequest)
	server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPost, path: "/api/v1/auth/mfa/totp/" + enrollment.EnrollmentID + "/confirm",
		body: fmt.Sprintf(`{"code": %q}`, totpCode(t, enrollment.Secret, time.Now()))}, http.StatusOK)

	var challenge struct{ ChallengeToken string }
	body = server.call(t, http.MethodPost, "/api/v1/auth/login", loginBody("admin", "a long password", "Laptop"), http.StatusAccepted)
	require.NoError(t, json.Unmarshal(body, &challenge))
	server.call(t, http.MethodPost, "/api/v1/auth/login/mfa", mfaBody(challenge.ChallengeToken, "totp", "000000"), http.StatusUnauthorized)
	right := totpCode(t, enrollment.Secret, time.Now().Add(30*time.Second))
	server.call(t, http.MethodPost, "/api/v1/auth/login/mfa", mfaBody(challenge.ChallengeToken, "totp", right), http.StatusUnauthorized)
}
