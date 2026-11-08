package login

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
)

type fakeFactors struct {
	enabled    bool
	totp       string
	backup     string
	err        error
	enableErr  error
	methodsErr error
}

func (fake *fakeFactors) HasSecondFactor(context.Context, uuid.UUID) (bool, error) {
	return fake.enabled, fake.enableErr
}

func (fake *fakeFactors) Methods(context.Context, uuid.UUID) ([]string, error) {
	return []string{"totp", "backup_code"}, fake.methodsErr
}

func (fake *fakeFactors) VerifyTOTP(_ context.Context, _ uuid.UUID, code string) (bool, error) {
	return code == fake.totp, fake.err
}

func (fake *fakeFactors) UseBackupCode(_ context.Context, _ uuid.UUID, code string) (bool, error) {
	matched := code == fake.backup
	fake.backup = "" // single use
	return matched, fake.err
}

// fakeChallenges issues numbered tokens and redeems each once.
type fakeChallenges struct {
	issued    map[string]uuid.UUID
	issueErr  error
	redeemErr error
}

func newFakeChallenges() *fakeChallenges { return &fakeChallenges{issued: map[string]uuid.UUID{}} }

func (fake *fakeChallenges) Issue(_ context.Context, userID uuid.UUID, purpose tokens.Purpose) (string, error) {
	if purpose != tokens.MFALogin {
		return "", errors.New("wrong purpose")
	}
	token := "challenge-" + userID.String()
	fake.issued[token] = userID
	return token, fake.issueErr
}

func (fake *fakeChallenges) Redeem(_ context.Context, _ tokens.Purpose, token string) (uuid.UUID, error) {
	if fake.redeemErr != nil {
		return uuid.Nil, fake.redeemErr
	}
	userID, ok := fake.issued[token]
	if !ok {
		return uuid.Nil, tokens.ErrInvalidToken
	}
	delete(fake.issued, token)
	return userID, nil
}

func mfaFixture() (fixture, *fakeFactors) {
	factors := &fakeFactors{enabled: true, totp: "123456", backup: "ABCDE-FGHIJ"}
	return newFixture(func(d *Dependencies) { d.SecondFactors = factors }), factors
}

func challengeFor(t *testing.T, setup fixture) string {
	t.Helper()
	result, err := setup.service.Login(t.Context(), attempt("alice", "right password"))
	require.NoError(t, err)
	require.NotEmpty(t, result.MFAChallenge)
	return result.MFAChallenge
}

func TestPasswordAloneIsNotEnoughWithASecondFactor(t *testing.T) {
	setup, _ := mfaFixture()

	result, err := setup.service.Login(t.Context(), attempt("alice", "right password"))

	require.NoError(t, err)
	assert.NotEmpty(t, result.MFAChallenge)
	assert.Equal(t, []string{"totp", "backup_code"}, result.MFAMethods)
	assert.Empty(t, result.Token, "no session before the second factor")
}

func TestCompleteMFA(t *testing.T) {
	for _, method := range []struct{ name, code string }{{MethodTOTP, "123456"}, {MethodBackupCode, "ABCDE-FGHIJ"}} {
		t.Run(method.name, func(t *testing.T) {
			setup, _ := mfaFixture()
			challenge := challengeFor(t, setup)

			result, err := setup.service.CompleteMFA(t.Context(), MFARequest{Challenge: challenge, Method: method.name, Code: method.code, ClientKey: "c"})

			require.NoError(t, err)
			assert.Equal(t, "nxs_token", result.Token)
			assert.Equal(t, "alice", result.User.Username)
		})
	}
}

func TestEachChallengeAllowsOneAttempt(t *testing.T) {
	setup, _ := mfaFixture()
	challenge := challengeFor(t, setup)

	_, err := setup.service.CompleteMFA(t.Context(), MFARequest{Challenge: challenge, Method: MethodTOTP, Code: "000000", ClientKey: "c"})
	require.ErrorIs(t, err, ErrInvalidSecondFactor)

	_, err = setup.service.CompleteMFA(t.Context(), MFARequest{Challenge: challenge, Method: MethodTOTP, Code: "123456", ClientKey: "c"})
	assert.ErrorIs(t, err, ErrInvalidSecondFactor, "the right code no longer helps: log in again")
}

func TestUnknownMethodIsRejected(t *testing.T) {
	setup, _ := mfaFixture()

	_, err := setup.service.CompleteMFA(t.Context(), MFARequest{Challenge: challengeFor(t, setup), Method: "sms", Code: "123456", ClientKey: "c"})

	assert.ErrorIs(t, err, ErrInvalidSecondFactor)
}

func TestCompleteMFAFailures(t *testing.T) {
	failure := errors.New("failure")

	limited := newFixture(func(d *Dependencies) { d.Limiter = fakeLimiter{deny: true} })
	_, err := limited.service.CompleteMFA(t.Context(), MFARequest{})
	require.ErrorIs(t, err, ErrRateLimited)

	challenges := newFakeChallenges()
	challenges.redeemErr = failure
	broken := newFixture(func(d *Dependencies) { d.Challenges = challenges })
	_, err = broken.service.CompleteMFA(t.Context(), MFARequest{Challenge: "x"})
	require.ErrorIs(t, err, failure)

	setup, factors := mfaFixture()
	challenge := challengeFor(t, setup)
	factors.err = failure
	_, err = setup.service.CompleteMFA(t.Context(), MFARequest{Challenge: challenge, Method: MethodTOTP, Code: "123456"})
	require.ErrorIs(t, err, failure)

	setup, _ = mfaFixture()
	challenge = challengeFor(t, setup)
	setup.accounts.lookupErr = failure
	_, err = setup.service.CompleteMFA(t.Context(), MFARequest{Challenge: challenge, Method: MethodTOTP, Code: "123456"})
	assert.ErrorIs(t, err, failure)
}

func TestLoginFailsWhenSecondFactorsCannotBeRead(t *testing.T) {
	failure := errors.New("failure")
	setup := newFixture(func(d *Dependencies) { d.SecondFactors = &fakeFactors{enableErr: failure} })
	_, err := setup.service.Login(t.Context(), attempt("alice", "right password"))
	require.ErrorIs(t, err, failure)

	challenges := newFakeChallenges()
	challenges.issueErr = failure
	setup = newFixture(func(d *Dependencies) {
		d.SecondFactors = &fakeFactors{enabled: true}
		d.Challenges = challenges
	})
	_, err = setup.service.Login(t.Context(), attempt("alice", "right password"))
	assert.ErrorIs(t, err, failure)
}

func TestLoginFailsWhenMethodsCannotBeListed(t *testing.T) {
	setup := newFixture(func(d *Dependencies) { d.SecondFactors = &fakeFactors{enabled: true, methodsErr: errors.New("down")} })

	_, err := setup.service.Login(t.Context(), attempt("alice", "right password"))

	assert.Error(t, err)
}

func TestStartSessionAfterSecurityKey(t *testing.T) {
	setup, _ := mfaFixture()
	challenge := challengeFor(t, setup)

	userID, err := setup.service.RedeemChallenge(t.Context(), challenge, "c")
	require.NoError(t, err)
	result, err := setup.service.StartSession(t.Context(), userID, attempt("", "").Device)

	require.NoError(t, err)
	assert.Equal(t, "nxs_token", result.Token)
	_, err = setup.service.RedeemChallenge(t.Context(), challenge, "c")
	assert.ErrorIs(t, err, ErrInvalidSecondFactor, "redeemed once")
}
