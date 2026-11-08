package oidc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	registrationstore "github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

func TestFirstSignInCreatesAnAccountAndLogsIn(t *testing.T) {
	fixture := newSignInFixture(t)
	flowID, code := fixture.signIn(t, nil, "")

	result, err := fixture.complete(t, flowID, code)

	require.NoError(t, err)
	require.Len(t, fixture.accounts.created, 1)
	created := fixture.accounts.created[0]
	assert.Equal(t, "ada", created.User.Username)
	assert.Equal(t, "Ada Lovelace", created.User.DisplayName)
	assert.Equal(t, "ada@example.com", created.User.Email)
	assert.Nil(t, created.User.PasswordHash)
	require.NotNil(t, created.User.EmailVerifiedAt)
	assert.Equal(t, testNow, *created.User.EmailVerifiedAt)
	assert.Nil(t, created.InviteHash)
	identity := fixture.store.identities[[2]string{"company", "sub-1"}]
	assert.Equal(t, created.User.ID, identity.UserID)
	assert.Equal(t, []uuid.UUID{created.User.ID}, fixture.logins.loggedIn)
	assert.Equal(t, []session.Device{{Name: "Phone"}}, fixture.logins.devices)
	assert.Equal(t, "nxs_token", result.Token)

	_, err = fixture.complete(t, flowID, code)
	assert.ErrorIs(t, err, ErrInvalidFlow, "a flow completes once")
}

func TestLaterSignInsLogIntoTheLinkedAccount(t *testing.T) {
	fixture := newSignInFixture(t)
	existing := uuid.New()
	fixture.store.identities[[2]string{"company", "sub-1"}] = store.OidcIdentity{ProviderID: "company", Subject: "sub-1", UserID: existing}
	fixture.logins.result = login.Result{MFAChallenge: "challenge", MFAMethods: []string{"totp"}}
	flowID, code := fixture.signIn(t, nil, "")

	result, err := fixture.complete(t, flowID, code)

	require.NoError(t, err)
	assert.Empty(t, fixture.accounts.created)
	assert.Equal(t, []uuid.UUID{existing}, fixture.logins.loggedIn)
	assert.Equal(t, "challenge", result.MFAChallenge, "second factors still apply")
}

func TestCompleteNeedsTheMatchingFlowAndCode(t *testing.T) {
	fixture := newSignInFixture(t)
	flowID, code := fixture.signIn(t, nil, "")

	_, err := fixture.complete(t, uuid.New(), code)
	require.ErrorIs(t, err, ErrInvalidFlow, "a code from someone else's flow")
	_, err = fixture.complete(t, flowID, "wrong")
	require.ErrorIs(t, err, ErrInvalidFlow)

	fixture.clock.now = testNow.Add(11 * time.Minute)
	_, err = fixture.complete(t, flowID, code)
	assert.ErrorIs(t, err, ErrInvalidFlow, "expired")
}

func TestALinkFlowCannotLogIn(t *testing.T) {
	fixture := newSignInFixture(t)
	user := uuid.New()
	flowID, code := fixture.signIn(t, &user, "")

	_, err := fixture.complete(t, flowID, code)

	require.ErrorIs(t, err, ErrInvalidFlow)
	assert.Empty(t, fixture.logins.loggedIn)
}

func TestCompleteErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(*signInFixture)
		want   error
	}{
		{"rate limited", func(f *signInFixture) { f.limiter.denied = true }, ErrRateLimited},
		{"consume fails", func(f *signInFixture) { f.store.failOn["ConsumeFlow"] = errFailure }, errFailure},
		{"lookup fails", func(f *signInFixture) { f.store.failOn["GetIdentity"] = errFailure }, errFailure},
		{"login fails", func(f *signInFixture) { f.logins.err = errFailure }, errFailure},
		{"provider gone", func(f *signInFixture) { f.store.failOn["GetProvider"] = errFailure }, errFailure},
		{"sign-up off", func(f *signInFixture) {
			provider := f.store.providers["company"]
			provider.AllowSignup = false
			f.store.providers["company"] = provider
		}, ErrNoAccount},
		{"registration closed", func(f *signInFixture) { f.settings.mode = "closed" }, ErrRegistrationClosed},
		{"invite needed", func(f *signInFixture) { f.settings.mode = "invite_only" }, ErrInviteRequired},
		{"mode unreadable", func(f *signInFixture) { f.settings.modeErr = errFailure }, errFailure},
		{"username check fails", func(f *signInFixture) { f.store.failOn["UsernameExists"] = errFailure }, errFailure},
		{"invite used up", func(f *signInFixture) { f.accounts.err = registrationstore.ErrInvalidInvite }, ErrInvalidInvite},
		{"email taken", func(f *signInFixture) { f.accounts.err = registrationstore.ErrEmailTaken }, ErrEmailInUse},
		{"signed up twice at once", func(f *signInFixture) { f.accounts.err = duplicate(store.IdentityKey) }, ErrIdentityInUse},
		{"saving fails", func(f *signInFixture) { f.accounts.err = errFailure }, errFailure},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSignInFixture(t)
			flowID, code := fixture.signIn(t, nil, "")
			test.change(&fixture)

			_, err := fixture.complete(t, flowID, code)

			assert.ErrorIs(t, err, test.want)
		})
	}
}

func TestSignUpNeedsAPlainEmail(t *testing.T) {
	for _, email := range []string{"", "Ada <ada@example.com>", "not-an-address"} {
		fixture := newSignInFixture(t)
		fixture.connector.claims.Email = email
		flowID, code := fixture.signIn(t, nil, "")

		_, err := fixture.complete(t, flowID, code)

		assert.ErrorIs(t, err, ErrEmailRequired, email)
	}
}

func TestCompleteRejectsDamagedClaims(t *testing.T) {
	fixture := newSignInFixture(t)
	flowID, code := fixture.signIn(t, nil, "")
	flow := fixture.store.flows[flowID]
	flow.Claims = []byte("{")
	fixture.store.flows[flowID] = flow

	_, err := fixture.complete(t, flowID, code)

	assert.ErrorContains(t, err, "reading sign-in result")
}

func TestInviteOnlySignUpUsesTheInvite(t *testing.T) {
	fixture := newSignInFixture(t)
	fixture.settings.mode = "invite_only"
	flowID, code := fixture.signIn(t, nil, "CODE1234")

	_, err := fixture.complete(t, flowID, code)

	require.NoError(t, err)
	assert.NotNil(t, fixture.accounts.created[0].InviteHash)
}

func TestUnverifiedEmailStaysUnverified(t *testing.T) {
	fixture := newSignInFixture(t)
	fixture.connector.claims.EmailVerified = false
	flowID, code := fixture.signIn(t, nil, "")

	_, err := fixture.complete(t, flowID, code)

	require.NoError(t, err)
	assert.Nil(t, fixture.accounts.created[0].User.EmailVerifiedAt)
}

func TestLinkingAddsTheIdentityToTheAccount(t *testing.T) {
	fixture := newSignInFixture(t)
	user := uuid.New()
	flowID, code := fixture.signIn(t, &user, "")

	require.NoError(t, fixture.service.CompleteLink(t.Context(), user, flowID, code))

	identity := fixture.store.identities[[2]string{"company", "sub-1"}]
	assert.Equal(t, user, identity.UserID)
	assert.Equal(t, "ada@example.com", identity.Email)
	assert.Equal(t, testNow, identity.CreatedAt)
}

func TestLinkingTwiceToTheSameAccountIsFine(t *testing.T) {
	fixture := newSignInFixture(t)
	user := uuid.New()
	fixture.store.identities[[2]string{"company", "sub-1"}] = store.OidcIdentity{ProviderID: "company", Subject: "sub-1", UserID: user}
	flowID, code := fixture.signIn(t, &user, "")

	assert.NoError(t, fixture.service.CompleteLink(t.Context(), user, flowID, code))
}

func TestLinkErrors(t *testing.T) {
	user := uuid.New()
	tests := []struct {
		name   string
		linkAs *uuid.UUID
		change func(*signInFixture)
		want   error
	}{
		{"a sign-in flow", nil, func(*signInFixture) {}, ErrInvalidFlow},
		{"someone else's flow", &[]uuid.UUID{uuid.New()}[0], func(*signInFixture) {}, ErrInvalidFlow},
		{"consume fails", &user, func(f *signInFixture) { f.store.failOn["ConsumeFlow"] = errFailure }, errFailure},
		{"belongs to another account", &user, func(f *signInFixture) {
			f.store.identities[[2]string{"company", "sub-1"}] = store.OidcIdentity{UserID: uuid.New()}
		}, ErrIdentityInUse},
		{"lookup fails", &user, func(f *signInFixture) { f.store.failOn["GetIdentity"] = errFailure }, errFailure},
		{"another login linked", &user, func(f *signInFixture) { f.store.duplicateOn = store.IdentityPerUserKey }, ErrAlreadyLinked},
		{"linked at the same moment", &user, func(f *signInFixture) { f.store.duplicateOn = store.IdentityKey }, ErrIdentityInUse},
		{"saving fails", &user, func(f *signInFixture) { f.store.failOn["InsertIdentity"] = errFailure }, errFailure},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSignInFixture(t)
			flowID, code := fixture.signIn(t, test.linkAs, "")
			test.change(&fixture)

			err := fixture.service.CompleteLink(t.Context(), user, flowID, code)

			assert.ErrorIs(t, err, test.want)
		})
	}
}
