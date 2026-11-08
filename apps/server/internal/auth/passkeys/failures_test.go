package passkeys

import (
	"errors"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errFailure = errors.New("failure")

// failingParty makes every library call fail.
type failingParty struct{}

func (failingParty) BeginRegistration(webauthn.User, ...webauthn.RegistrationOption) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	return nil, nil, errFailure
}
func (failingParty) CreateCredential(webauthn.User, webauthn.SessionData, *protocol.ParsedCredentialCreationData) (*webauthn.Credential, error) {
	return nil, errFailure
}
func (failingParty) BeginLogin(webauthn.User, ...webauthn.LoginOption) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	return nil, nil, errFailure
}
func (failingParty) ValidateLogin(webauthn.User, webauthn.SessionData, *protocol.ParsedCredentialAssertionData) (*webauthn.Credential, error) {
	return nil, errFailure
}
func (failingParty) BeginDiscoverableLogin(...webauthn.LoginOption) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	return nil, nil, errFailure
}
func (failingParty) ValidatePasskeyLogin(webauthn.DiscoverableUserHandler, webauthn.SessionData, *protocol.ParsedCredentialAssertionData) (webauthn.User, *webauthn.Credential, error) {
	return nil, nil, errFailure
}

func TestLibraryFailuresArePassedOn(t *testing.T) {
	setup := defaultFixture()
	registered(t, setup, KindSecurityKey)
	setup.service.dependencies.newParty = func(*webauthn.Config) (relyingParty, error) { return failingParty{}, nil }

	_, err := setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "x")
	require.ErrorIs(t, err, errFailure)
	_, err = setup.service.BeginSecondFactor(t.Context(), alice.ID)
	require.ErrorIs(t, err, errFailure)
	_, err = setup.service.BeginPasskeyLogin(t.Context())
	assert.ErrorIs(t, err, errFailure)
}

func TestStoreFailuresArePassedOn(t *testing.T) {
	steps := map[string]func(*testing.T, fixture) error{
		"ListCredentials": func(t *testing.T, setup fixture) error {
			_, err := setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "x")
			return err
		},
		"InsertCeremony": func(t *testing.T, setup fixture) error {
			_, err := setup.service.BeginPasskeyLogin(t.Context())
			return err
		},
		"TakeCeremony": func(t *testing.T, setup fixture) error {
			_, err := setup.service.FinishPasskeyLogin(t.Context(), uuid.New(), nil)
			return err
		},
		"DeleteCredential": func(t *testing.T, setup fixture) error {
			return setup.service.Remove(t.Context(), alice.ID, uuid.New())
		},
	}
	for method, step := range steps {
		t.Run(method, func(t *testing.T) {
			setup := defaultFixture()
			setup.store.fail, setup.store.failOn = errFailure, method

			assert.ErrorIs(t, step(t, setup), errFailure)
		})
	}

	setup := defaultFixture()
	setup.store.fail, setup.store.failOn = errFailure, "ListCredentials"
	_, err := setup.service.List(t.Context(), alice.ID)
	assert.ErrorIs(t, err, errFailure)
}

func TestSavingFailures(t *testing.T) {
	for _, method := range []string{"InsertCredential", "UpdateCredentialAfterUse"} {
		t.Run(method, func(t *testing.T) {
			setup := defaultFixture()
			authenticator := newSoftwareAuthenticator(t)
			started, err := setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "x")
			require.NoError(t, err)
			response := authenticator.register(t, started.Options, testOrigin)
			if method == "InsertCredential" {
				setup.store.fail, setup.store.failOn = errFailure, method
			}
			err = setup.service.FinishRegistration(t.Context(), Answer{UserID: alice.ID, CeremonyID: started.CeremonyID, Kind: KindPasskey, Response: response})
			if method == "InsertCredential" {
				require.ErrorIs(t, err, errFailure)
				return
			}
			require.NoError(t, err)
			authenticator.userHandle = alice.ID[:]
			login, err := setup.service.BeginPasskeyLogin(t.Context())
			require.NoError(t, err)
			setup.store.fail, setup.store.failOn = errFailure, method
			_, err = setup.service.FinishPasskeyLogin(t.Context(), login.CeremonyID, authenticator.assert(t, login.Options))
			assert.ErrorIs(t, err, errFailure)
		})
	}
}

func TestFinishFailsWhenTheDomainOrAccountBreaksMeanwhile(t *testing.T) {
	setup := defaultFixture()
	authenticator := registered(t, setup, KindSecurityKey)

	second, err := setup.service.BeginSecondFactor(t.Context(), alice.ID)
	require.NoError(t, err)
	setup.users.err = errFailure
	_, err = setup.service.FinishSecondFactor(t.Context(), second.CeremonyID, authenticator.assert(t, second.Options))
	require.ErrorIs(t, err, errFailure, "account lookup fails during finish")
	setup.users.err = nil

	registration, err := setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "x")
	require.NoError(t, err)
	response := newSoftwareAuthenticator(t).register(t, registration.Options, testOrigin)
	setup.users.err = errFailure
	err = setup.service.FinishRegistration(t.Context(), Answer{UserID: alice.ID, CeremonyID: registration.CeremonyID, Kind: KindPasskey, Response: response})
	require.ErrorIs(t, err, errFailure)
	setup.users.err = nil

	for name, finish := range map[string]func(Started) error{
		"registration": func(started Started) error {
			return setup.service.FinishRegistration(t.Context(), Answer{UserID: alice.ID, CeremonyID: started.CeremonyID, Kind: KindPasskey,
				Response: newSoftwareAuthenticator(t).register(t, started.Options, testOrigin)})
		},
		"second factor": func(started Started) error {
			_, err := setup.service.FinishSecondFactor(t.Context(), started.CeremonyID, authenticator.assert(t, started.Options))
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			var started Started
			var err error
			if name == "registration" {
				started, err = setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "x")
			} else {
				started, err = setup.service.BeginSecondFactor(t.Context(), alice.ID)
			}
			require.NoError(t, err)
			setup.domain.err = errFailure
			defer func() { setup.domain.err = nil }()

			assert.ErrorIs(t, finish(started), errFailure)
		})
	}
}

func TestBeginFailsWithoutAccount(t *testing.T) {
	setup := defaultFixture()
	setup.users.err = errFailure

	_, err := setup.service.BeginSecondFactor(t.Context(), alice.ID)
	require.ErrorIs(t, err, errFailure)
	_, err = setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "x")
	assert.ErrorIs(t, err, errFailure)
}

func TestSecondFactorRejectsBadAnswers(t *testing.T) {
	setup := defaultFixture()
	registered(t, setup, KindSecurityKey)
	started, err := setup.service.BeginSecondFactor(t.Context(), alice.ID)
	require.NoError(t, err)
	_, err = setup.service.FinishSecondFactor(t.Context(), started.CeremonyID, []byte("{}"))
	require.ErrorIs(t, err, ErrVerificationFails)

	stranger := newSoftwareAuthenticator(t)
	started, err = setup.service.BeginSecondFactor(t.Context(), alice.ID)
	require.NoError(t, err)
	_, err = setup.service.FinishSecondFactor(t.Context(), started.CeremonyID, stranger.assert(t, started.Options))
	require.ErrorIs(t, err, ErrVerificationFails, "a key alice never registered")

	started, err = setup.service.BeginPasskeyLogin(t.Context())
	require.NoError(t, err)
	_, err = setup.service.FinishPasskeyLogin(t.Context(), started.CeremonyID, []byte("{}"))
	assert.ErrorIs(t, err, ErrVerificationFails)
}

func TestCorruptStoredDataIsAnError(t *testing.T) {
	setup := defaultFixture()
	registered(t, setup, KindSecurityKey)
	for id, row := range setup.store.credentials {
		row.Credential = []byte("{")
		setup.store.credentials[id] = row
	}
	_, err := setup.service.BeginSecondFactor(t.Context(), alice.ID)
	require.ErrorContains(t, err, "decoding stored credential")

	clean := defaultFixture()
	started, err := clean.service.BeginPasskeyLogin(t.Context())
	require.NoError(t, err)
	row := clean.store.ceremonies[started.CeremonyID]
	row.Session = []byte("{")
	clean.store.ceremonies[started.CeremonyID] = row
	_, err = clean.service.FinishPasskeyLogin(t.Context(), started.CeremonyID, nil)
	assert.ErrorContains(t, err, "decoding WebAuthn session")
}

func TestBeginSecondFactorNeedsADomain(t *testing.T) {
	_, err := newFixture(fixedDomain{}).service.BeginSecondFactor(t.Context(), alice.ID)

	assert.ErrorIs(t, err, ErrNoDomain)
}

func TestNewServiceKeepsAGivenRelyingParty(t *testing.T) {
	service := NewService(Dependencies{
		Domain:   fixedDomain{domain: testRPID},
		newParty: func(*webauthn.Config) (relyingParty, error) { return failingParty{}, nil },
	})

	_, err := service.BeginPasskeyLogin(t.Context())

	assert.ErrorIs(t, err, errFailure)
}
