package oidc

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
)

func TestIdentitiesAndUnlink(t *testing.T) {
	fixture := newSignInFixture(t)
	user := uuid.New()
	fixture.store.identities[[2]string{"company", "sub-1"}] = store.OidcIdentity{
		ProviderID: "company", Subject: "sub-1", UserID: user, Email: "ada@example.com", CreatedAt: testNow,
	}

	identities, err := fixture.service.Identities(t.Context(), user)
	require.NoError(t, err)
	assert.Equal(t, []LinkedIdentity{{ProviderID: "company", DisplayName: "Company login", Email: "ada@example.com", LinkedAt: testNow}}, identities)

	require.ErrorIs(t, fixture.service.Unlink(t.Context(), user, "company"), ErrLastLoginMethod)
	require.ErrorIs(t, fixture.service.Unlink(t.Context(), user, "other"), ErrNotLinked)

	fixture.store.otherMethods = 1
	require.NoError(t, fixture.service.Unlink(t.Context(), user, "company"))
	require.ErrorIs(t, fixture.service.Unlink(t.Context(), user, "company"), ErrNotLinked)
	identities, err = fixture.service.Identities(t.Context(), user)
	require.NoError(t, err)
	assert.Empty(t, identities)
}

func TestUnlinkErrors(t *testing.T) {
	user := uuid.New()
	for method, others := range map[string]int32{"CountOtherLoginMethods": 1, "ListIdentitiesForUser": 0, "DeleteIdentity": 1} {
		t.Run(method, func(t *testing.T) {
			fixture := newSignInFixture(t)
			fixture.store.otherMethods = others
			fixture.store.failOn[method] = errFailure

			assert.ErrorIs(t, fixture.service.Unlink(t.Context(), user, "company"), errFailure)
		})
	}
	fixture := newSignInFixture(t)
	fixture.store.failOn["ListIdentitiesForUser"] = errFailure
	_, err := fixture.service.Identities(t.Context(), user)
	assert.ErrorIs(t, err, errFailure)
}
