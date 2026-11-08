package passkeys

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
)

var alice = accountsstore.User{ID: uuid.MustParse("01900000-0000-7000-8000-0000000000a1"), Username: "alice", DisplayName: "Alice"}

type fixture struct {
	service *Service
	store   *fakeStore
	clock   *movableClock
	users   *fakeUsers
	domain  *switchableDomain
}

func newFixture(domain fixedDomain) fixture {
	fake := newFakeStore()
	clock := &movableClock{now: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)}
	users := &fakeUsers{users: map[uuid.UUID]accountsstore.User{alice.ID: alice}}
	switchable := &switchableDomain{fixedDomain: domain}
	service := NewService(Dependencies{Store: fake, Users: users, Domain: switchable, Clock: clock})
	return fixture{service: service, store: fake, clock: clock, users: users, domain: switchable}
}

func defaultFixture() fixture { return newFixture(fixedDomain{domain: testRPID}) }

// registered adds a credential of kind for alice and returns its authenticator.
func registered(t *testing.T, setup fixture, kind string) *softwareAuthenticator {
	t.Helper()
	authenticator := newSoftwareAuthenticator(t)
	started, err := setup.service.BeginRegistration(t.Context(), alice.ID, kind, "My key")
	require.NoError(t, err)
	response := authenticator.register(t, started.Options, testOrigin)
	require.NoError(t, setup.service.FinishRegistration(t.Context(), Answer{UserID: alice.ID, CeremonyID: started.CeremonyID, Kind: kind, Response: response}))
	authenticator.userHandle = alice.ID[:]
	return authenticator
}

func TestRegisterAndUseASecurityKey(t *testing.T) {
	setup := defaultFixture()
	authenticator := registered(t, setup, KindSecurityKey)

	listed, err := setup.service.List(t.Context(), alice.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "My key", listed[0].Name)
	assert.Equal(t, KindSecurityKey, listed[0].Kind)
	assert.Nil(t, listed[0].LastUsedAt)

	started, err := setup.service.BeginSecondFactor(t.Context(), alice.ID)
	require.NoError(t, err)
	userID, err := setup.service.FinishSecondFactor(t.Context(), started.CeremonyID, authenticator.assert(t, started.Options))

	require.NoError(t, err)
	assert.Equal(t, alice.ID, userID)
	listed, err = setup.service.List(t.Context(), alice.ID)
	require.NoError(t, err)
	assert.NotNil(t, listed[0].LastUsedAt)
}

func TestPasskeyLoginWithoutAPassword(t *testing.T) {
	setup := defaultFixture()
	authenticator := registered(t, setup, KindPasskey)

	started, err := setup.service.BeginPasskeyLogin(t.Context())
	require.NoError(t, err)
	userID, err := setup.service.FinishPasskeyLogin(t.Context(), started.CeremonyID, authenticator.assert(t, started.Options))

	require.NoError(t, err)
	assert.Equal(t, alice.ID, userID)
}

func TestPasskeyLoginRejectsUnknownOwners(t *testing.T) {
	setup := defaultFixture()
	authenticator := registered(t, setup, KindPasskey)

	for name, handle := range map[string][]byte{"not a uuid": []byte("x"), "unknown account": func() []byte { id := uuid.New(); return id[:] }()} {
		t.Run(name, func(t *testing.T) {
			authenticator.userHandle = handle
			started, err := setup.service.BeginPasskeyLogin(t.Context())
			require.NoError(t, err)

			_, err = setup.service.FinishPasskeyLogin(t.Context(), started.CeremonyID, authenticator.assert(t, started.Options))

			assert.ErrorIs(t, err, ErrVerificationFails)
		})
	}
}

func TestCeremoniesAreSingleUseAndExpire(t *testing.T) {
	setup := defaultFixture()
	authenticator := registered(t, setup, KindSecurityKey)
	started, err := setup.service.BeginSecondFactor(t.Context(), alice.ID)
	require.NoError(t, err)
	answer := authenticator.assert(t, started.Options)

	_, err = setup.service.FinishSecondFactor(t.Context(), started.CeremonyID, answer)
	require.NoError(t, err)
	_, err = setup.service.FinishSecondFactor(t.Context(), started.CeremonyID, answer)
	require.ErrorIs(t, err, ErrCeremonyNotFound)

	late, err := setup.service.BeginSecondFactor(t.Context(), alice.ID)
	require.NoError(t, err)
	setup.clock.now = setup.clock.now.Add(6 * time.Minute)
	_, err = setup.service.FinishSecondFactor(t.Context(), late.CeremonyID, authenticator.assert(t, late.Options))
	assert.ErrorIs(t, err, ErrCeremonyNotFound)
}

func TestWrongOriginOrResponseIsRejected(t *testing.T) {
	setup := defaultFixture()
	authenticator := newSoftwareAuthenticator(t)
	started, err := setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "Phishing")
	require.NoError(t, err)

	err = setup.service.FinishRegistration(t.Context(), Answer{UserID: alice.ID, CeremonyID: started.CeremonyID, Kind: KindPasskey,
		Response: authenticator.register(t, started.Options, "https://evil.example.net")})
	require.ErrorIs(t, err, ErrVerificationFails, "a response made for another site must fail")

	again, err := setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "Broken")
	require.NoError(t, err)
	err = setup.service.FinishRegistration(t.Context(), Answer{UserID: alice.ID, CeremonyID: again.CeremonyID, Kind: KindPasskey, Response: []byte("{}")})
	assert.ErrorIs(t, err, ErrVerificationFails)
}

func TestCeremoniesAreBoundToPurposeAndUser(t *testing.T) {
	setup := defaultFixture()
	authenticator := newSoftwareAuthenticator(t)
	started, err := setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "Phone")
	require.NoError(t, err)
	response := authenticator.register(t, started.Options, testOrigin)

	err = setup.service.FinishRegistration(t.Context(), Answer{UserID: alice.ID, CeremonyID: started.CeremonyID, Kind: KindSecurityKey, Response: response})
	require.ErrorIs(t, err, ErrCeremonyNotFound, "a passkey ceremony cannot register a security key")

	other, err := setup.service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "Phone")
	require.NoError(t, err)
	err = setup.service.FinishRegistration(t.Context(), Answer{UserID: uuid.New(), CeremonyID: other.CeremonyID, Kind: KindPasskey, Response: response})
	assert.ErrorIs(t, err, ErrCeremonyNotFound, "another user cannot finish alice's ceremony")
}

func TestRegistrationNeedsAKnownKindAndADomain(t *testing.T) {
	_, err := defaultFixture().service.BeginRegistration(t.Context(), alice.ID, "fingerprint", "x")
	require.ErrorIs(t, err, ErrUnknownKind)
	require.ErrorIs(t, defaultFixture().service.FinishRegistration(t.Context(), Answer{Kind: "fingerprint"}), ErrUnknownKind)

	_, err = newFixture(fixedDomain{}).service.BeginRegistration(t.Context(), alice.ID, KindPasskey, "x")
	require.ErrorIs(t, err, ErrNoDomain)
	_, err = newFixture(fixedDomain{err: errors.New("down")}).service.BeginPasskeyLogin(t.Context())
	require.ErrorContains(t, err, "down")
	_, err = newFixture(fixedDomain{domain: "not a domain"}).service.BeginPasskeyLogin(t.Context())
	assert.ErrorContains(t, err, "configuring WebAuthn")
}

func TestSecondFactorNeedsACredential(t *testing.T) {
	_, err := defaultFixture().service.BeginSecondFactor(t.Context(), alice.ID)

	assert.ErrorIs(t, err, ErrNoCredentials)
}

func TestRemove(t *testing.T) {
	setup := defaultFixture()
	registered(t, setup, KindSecurityKey)
	listed, err := setup.service.List(t.Context(), alice.ID)
	require.NoError(t, err)

	require.ErrorIs(t, setup.service.Remove(t.Context(), uuid.New(), listed[0].ID), ErrNotFound)
	require.NoError(t, setup.service.Remove(t.Context(), alice.ID, listed[0].ID))
	listed, err = setup.service.List(t.Context(), alice.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)
}
