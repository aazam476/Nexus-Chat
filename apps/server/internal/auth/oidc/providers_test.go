package oidc

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/secretbox"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

var testNow = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func testBox(t *testing.T) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(bytes.Repeat([]byte{3}, 32))
	require.NoError(t, err)
	return box
}

type providersFixture struct {
	service *Providers
	store   *fakeStore
	checker *fakeChecker
}

func newProvidersFixture(t *testing.T) providersFixture {
	t.Helper()
	fixture := providersFixture{store: newFakeStore(), checker: &fakeChecker{}}
	fixture.service = NewProviders(fixture.store, testBox(t), fixture.checker, &movableClock{now: testNow})
	return fixture
}

func pointer(text string) *string { return &text }

func companyInput() ProviderInput {
	return ProviderInput{
		ID: "company", DisplayName: " Company login ", Issuer: " https://login.example.com/realms/staff ",
		ClientID: " nexus ", ClientSecret: pointer("s3cret"), AllowSignup: true,
	}
}

func TestCreateSavesAProviderWithItsSecretSealed(t *testing.T) {
	fixture := newProvidersFixture(t)

	created, err := fixture.service.Create(t.Context(), companyInput())

	require.NoError(t, err)
	assert.Equal(t, ProviderView{
		ID: "company", DisplayName: "Company login", Issuer: "https://login.example.com/realms/staff", ClientID: "nexus",
		ClientSecretSet: true, Scopes: []string{"openid", "email", "profile"}, AllowSignup: true,
	}, created)
	assert.Equal(t, []string{"https://login.example.com/realms/staff"}, fixture.checker.checked)
	stored := fixture.store.providers["company"]
	assert.NotContains(t, string(stored.ClientSecretSealed), "s3cret")
	provider, err := fixture.service.Get(t.Context(), "company")
	require.NoError(t, err)
	assert.Equal(t, "s3cret", provider.ClientSecret)
	listed, err := fixture.service.List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []ProviderView{created}, listed)
}

func TestCreateWithoutASecretMakesAPublicClient(t *testing.T) {
	for name, secret := range map[string]*string{"left out": nil, "empty": pointer("")} {
		t.Run(name, func(t *testing.T) {
			fixture := newProvidersFixture(t)
			input := companyInput()
			input.ClientSecret = secret

			created, err := fixture.service.Create(t.Context(), input)

			require.NoError(t, err)
			assert.False(t, created.ClientSecretSet)
			provider, err := fixture.service.Get(t.Context(), "company")
			require.NoError(t, err)
			assert.Empty(t, provider.ClientSecret)
		})
	}
}

func TestCreateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		change func(*ProviderInput)
		field  string
	}{
		{"id with capitals", func(input *ProviderInput) { input.ID = "Company" }, "id"},
		{"id too short", func(input *ProviderInput) { input.ID = "c" }, "id"},
		{"empty name", func(input *ProviderInput) { input.DisplayName = "  " }, "displayName"},
		{"long name", func(input *ProviderInput) { input.DisplayName = strings.Repeat("n", 65) }, "displayName"},
		{"plain http issuer", func(input *ProviderInput) { input.Issuer = "http://login.example.com" }, "issuer"},
		{"issuer with query", func(input *ProviderInput) { input.Issuer = "https://login.example.com/?a=b" }, "issuer"},
		{"issuer with fragment", func(input *ProviderInput) { input.Issuer = "https://login.example.com/#a" }, "issuer"},
		{"issuer with user", func(input *ProviderInput) { input.Issuer = "https://me@login.example.com" }, "issuer"},
		{"issuer without host", func(input *ProviderInput) { input.Issuer = "https:///path" }, "issuer"},
		{"issuer not a URL", func(input *ProviderInput) { input.Issuer = "https://bad host\x7f" }, "issuer"},
		{"issuer too long", func(input *ProviderInput) { input.Issuer = "https://example.com/" + strings.Repeat("a", 500) }, "issuer"},
		{"other scheme", func(input *ProviderInput) { input.Issuer = "ftp://login.example.com" }, "issuer"},
		{"empty client id", func(input *ProviderInput) { input.ClientID = "" }, "clientId"},
		{"client id with space", func(input *ProviderInput) { input.ClientID = "a b" }, "clientId"},
		{"long client id", func(input *ProviderInput) { input.ClientID = strings.Repeat("c", 257) }, "clientId"},
		{"long secret", func(input *ProviderInput) { input.ClientSecret = pointer(strings.Repeat("s", 1025)) }, "clientSecret"},
		{"scope with quote", func(input *ProviderInput) { input.Scopes = []string{`a"b`} }, "scopes"},
		{"empty scope", func(input *ProviderInput) { input.Scopes = []string{" "} }, "scopes"},
		{"too many scopes", func(input *ProviderInput) {
			input.Scopes = strings.Split("a b c d e f g h i j", " ")
		}, "scopes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProvidersFixture(t)
			input := companyInput()
			test.change(&input)

			_, err := fixture.service.Create(t.Context(), input)

			var problems *validation.Error
			require.ErrorAs(t, err, &problems)
			require.Len(t, problems.Fields, 1)
			assert.Equal(t, test.field, problems.Fields[0].Field)
			assert.Empty(t, fixture.checker.checked, "invalid settings are not fetched")
			assert.Empty(t, fixture.store.providers)
		})
	}
}

func TestIssuerMayUseHTTPOnLoopback(t *testing.T) {
	for _, issuer := range []string{"http://localhost:8080", "http://127.0.0.1:5556/dex", "http://[::1]:9000"} {
		assert.NoError(t, checkIssuer(issuer), issuer)
	}
}

func TestScopesAreNormalized(t *testing.T) {
	assert.Equal(t, []string{"openid", "email", "groups"}, normalizeScopes([]string{"email", " groups ", "openid", "email"}))
	assert.Equal(t, []string{"openid", "email", "profile"}, normalizeScopes(nil))
}

func TestCreateReportsAnIssuerWithoutConfiguration(t *testing.T) {
	fixture := newProvidersFixture(t)
	fixture.checker.err = errFailure

	_, err := fixture.service.Create(t.Context(), companyInput())

	var problems *validation.Error
	require.ErrorAs(t, err, &problems)
	assert.Equal(t, "issuer", problems.Fields[0].Field)
	assert.Contains(t, problems.Fields[0].Message, "could not load the provider's OpenID configuration")
}

func TestCreateReportsDuplicatesAndFailures(t *testing.T) {
	fixture := newProvidersFixture(t)
	fixture.store.duplicateOn = store.ProviderKey
	_, err := fixture.service.Create(t.Context(), companyInput())
	require.ErrorIs(t, err, ErrProviderExists)

	fixture = newProvidersFixture(t)
	fixture.store.failOn["InsertProvider"] = errFailure
	_, err = fixture.service.Create(t.Context(), companyInput())
	assert.ErrorIs(t, err, errFailure)
}

func TestUpdateKeepsReplacesOrRemovesTheSecret(t *testing.T) {
	fixture := newProvidersFixture(t)
	_, err := fixture.service.Create(t.Context(), companyInput())
	require.NoError(t, err)
	secretOf := func() string {
		provider, err := fixture.service.Get(t.Context(), "company")
		require.NoError(t, err)
		return provider.ClientSecret
	}

	input := companyInput()
	input.ID, input.ClientSecret, input.DisplayName = "ignored", nil, "Staff"
	updated, err := fixture.service.Update(t.Context(), "company", input)
	require.NoError(t, err)
	assert.Equal(t, "company", updated.ID, "the ID comes from the URL")
	assert.Equal(t, "Staff", updated.DisplayName)
	assert.Equal(t, "s3cret", secretOf())

	input.ClientSecret = pointer("new-secret")
	_, err = fixture.service.Update(t.Context(), "company", input)
	require.NoError(t, err)
	assert.Equal(t, "new-secret", secretOf())

	input.ClientSecret = pointer("")
	updated, err = fixture.service.Update(t.Context(), "company", input)
	require.NoError(t, err)
	assert.False(t, updated.ClientSecretSet)
	assert.Empty(t, secretOf())
}

func TestUpdateErrors(t *testing.T) {
	fixture := newProvidersFixture(t)
	_, err := fixture.service.Update(t.Context(), "missing", companyInput())
	require.ErrorIs(t, err, ErrProviderNotFound)

	_, err = fixture.service.Create(t.Context(), companyInput())
	require.NoError(t, err)
	invalid := companyInput()
	invalid.ClientID = ""
	_, err = fixture.service.Update(t.Context(), "company", invalid)
	var problems *validation.Error
	require.ErrorAs(t, err, &problems)

	fixture.store.deleteNothing = true
	_, err = fixture.service.Update(t.Context(), "company", companyInput())
	require.ErrorIs(t, err, ErrProviderNotFound, "deleted while being edited")

	fixture.store.failOn["UpdateProvider"] = errFailure
	_, err = fixture.service.Update(t.Context(), "company", companyInput())
	require.ErrorIs(t, err, errFailure)

	fixture.store.failOn["GetProvider"] = errFailure
	_, err = fixture.service.Update(t.Context(), "company", companyInput())
	assert.ErrorIs(t, err, errFailure)
}

func TestGetAndListErrors(t *testing.T) {
	fixture := newProvidersFixture(t)
	_, err := fixture.service.Get(t.Context(), "missing")
	require.ErrorIs(t, err, ErrProviderNotFound)

	fixture.store.providers["broken"] = store.OidcProvider{ID: "broken", ClientSecretSealed: []byte("not sealed")}
	_, err = fixture.service.Get(t.Context(), "broken")
	require.ErrorIs(t, err, secretbox.ErrCannotOpen)

	fixture.store.failOn["GetProvider"] = errFailure
	_, err = fixture.service.Get(t.Context(), "broken")
	require.ErrorIs(t, err, errFailure)

	fixture.store.failOn["ListProviders"] = errFailure
	_, err = fixture.service.List(t.Context())
	assert.ErrorIs(t, err, errFailure)
}

func TestDelete(t *testing.T) {
	fixture := newProvidersFixture(t)
	_, err := fixture.service.Create(t.Context(), companyInput())
	require.NoError(t, err)

	require.NoError(t, fixture.service.Delete(t.Context(), "company"))
	require.ErrorIs(t, fixture.service.Delete(t.Context(), "company"), ErrProviderNotFound)
	fixture.store.failOn["DeleteProvider"] = errFailure
	assert.ErrorIs(t, fixture.service.Delete(t.Context(), "company"), errFailure)
}
