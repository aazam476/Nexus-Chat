package oidc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
)

func createRequest() api.CreateOidcProviderRequestObject {
	return api.CreateOidcProviderRequestObject{Body: &api.CreateOidcProviderRequest{
		Id: "staff", DisplayName: "Staff", Issuer: "https://staff.example.com", ClientId: "client",
		ClientSecret: pointer("secret"), Scopes: &[]string{"groups"}, AllowSignup: false,
	}}
}

func updateRequest(id string) api.UpdateOidcProviderRequestObject {
	return api.UpdateOidcProviderRequestObject{ProviderId: id, Body: &api.UpdateOidcProviderRequest{
		DisplayName: "Staff login", Issuer: "https://staff.example.com", ClientId: "client", AllowSignup: true,
	}}
}

func TestAdminProviderLifecycle(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{admin: true})

	created, err := fixture.handler.CreateOidcProvider(fixture.ctx, createRequest())
	require.NoError(t, err)
	assert.Equal(t, api.CreateOidcProvider201JSONResponse{
		Id: "staff", DisplayName: "Staff", Issuer: "https://staff.example.com", ClientId: "client",
		ClientSecretSet: true, Scopes: []string{"openid", "groups"},
	}, created)

	updated, err := fixture.handler.UpdateOidcProvider(fixture.ctx, updateRequest("staff"))
	require.NoError(t, err)
	assert.Equal(t, api.UpdateOidcProvider200JSONResponse{
		Id: "staff", DisplayName: "Staff login", Issuer: "https://staff.example.com", ClientId: "client",
		ClientSecretSet: true, Scopes: []string{"openid", "email", "profile"}, AllowSignup: true,
	}, updated)

	listed, err := fixture.handler.ListOidcProviders(fixture.ctx, api.ListOidcProvidersRequestObject{})
	require.NoError(t, err)
	list := listed.(api.ListOidcProviders200JSONResponse) //nolint:forcetypeassert // the test fails otherwise
	assert.Len(t, list.Providers, 2)
	require.NotNil(t, list.CallbackUrl)
	assert.Equal(t, testBase+CallbackPath, *list.CallbackUrl)

	deleted, err := fixture.handler.DeleteOidcProvider(fixture.ctx, api.DeleteOidcProviderRequestObject{ProviderId: "staff"})
	require.NoError(t, err)
	assert.Equal(t, api.DeleteOidcProvider204Response{}, deleted)
}

func TestAdminProviderErrors(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{admin: true})
	invalid := createRequest()
	invalid.Body.Id = "Bad ID"
	response, err := fixture.handler.CreateOidcProvider(fixture.ctx, invalid)
	require.NoError(t, err)
	assert.IsType(t, api.CreateOidcProvider422JSONResponse{}, response)

	fixture.store.duplicateOn = store.ProviderKey
	response, err = fixture.handler.CreateOidcProvider(fixture.ctx, createRequest())
	require.NoError(t, err)
	assert.Equal(t, api.CreateOidcProvider409JSONResponse{Code: "provider_exists", Message: ErrProviderExists.Error()}, response)
	fixture.store.duplicateOn = ""

	fixture.store.failOn["InsertProvider"] = errFailure
	_, err = fixture.handler.CreateOidcProvider(fixture.ctx, createRequest())
	require.ErrorIs(t, err, errFailure)

	updated, err := fixture.handler.UpdateOidcProvider(fixture.ctx, updateRequest("missing"))
	require.NoError(t, err)
	assert.IsType(t, api.UpdateOidcProvider404JSONResponse{}, updated)
	invalidUpdate := updateRequest("company")
	invalidUpdate.Body.ClientId = ""
	updated, err = fixture.handler.UpdateOidcProvider(fixture.ctx, invalidUpdate)
	require.NoError(t, err)
	assert.IsType(t, api.UpdateOidcProvider422JSONResponse{}, updated)
	fixture.store.failOn["UpdateProvider"] = errFailure
	_, err = fixture.handler.UpdateOidcProvider(fixture.ctx, updateRequest("company"))
	require.ErrorIs(t, err, errFailure)

	deleted, err := fixture.handler.DeleteOidcProvider(fixture.ctx, api.DeleteOidcProviderRequestObject{ProviderId: "missing"})
	require.NoError(t, err)
	assert.IsType(t, api.DeleteOidcProvider404JSONResponse{}, deleted)
	fixture.store.failOn["DeleteProvider"] = errFailure
	_, err = fixture.handler.DeleteOidcProvider(fixture.ctx, api.DeleteOidcProviderRequestObject{ProviderId: "company"})
	require.ErrorIs(t, err, errFailure)
}

func TestListOidcProvidersWithoutADomainOrStore(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{admin: true})
	fixture.settings.base = ""
	listed, err := fixture.handler.ListOidcProviders(fixture.ctx, api.ListOidcProvidersRequestObject{})
	require.NoError(t, err)
	assert.Nil(t, listed.(api.ListOidcProviders200JSONResponse).CallbackUrl) //nolint:forcetypeassert // the test fails otherwise

	fixture.settings.baseErr = errFailure
	_, err = fixture.handler.ListOidcProviders(fixture.ctx, api.ListOidcProvidersRequestObject{})
	require.ErrorIs(t, err, errFailure)

	fixture.store.failOn["ListProviders"] = errFailure
	_, err = fixture.handler.ListOidcProviders(fixture.ctx, api.ListOidcProvidersRequestObject{})
	assert.ErrorIs(t, err, errFailure)
}

func TestAdminOperationsNeedAnInstanceAdmin(t *testing.T) {
	for name, users := range map[string]fakeUsers{"not admin": {}, "lookup fails": {err: errFailure}} {
		t.Run(name, func(t *testing.T) {
			fixture := newHandlerFixture(t, users)
			ctx := fixture.ctx
			list, listErr := fixture.handler.ListOidcProviders(ctx, api.ListOidcProvidersRequestObject{})
			create, createErr := fixture.handler.CreateOidcProvider(ctx, createRequest())
			update, updateErr := fixture.handler.UpdateOidcProvider(ctx, updateRequest("company"))
			remove, removeErr := fixture.handler.DeleteOidcProvider(ctx, api.DeleteOidcProviderRequestObject{ProviderId: "company"})

			if users.err != nil {
				for _, err := range []error{listErr, createErr, updateErr, removeErr} {
					require.ErrorIs(t, err, errFailure)
				}
				return
			}
			assert.IsType(t, api.ListOidcProviders403JSONResponse{}, list)
			assert.IsType(t, api.CreateOidcProvider403JSONResponse{}, create)
			assert.IsType(t, api.UpdateOidcProvider403JSONResponse{}, update)
			assert.IsType(t, api.DeleteOidcProvider403JSONResponse{}, remove)
			assert.Contains(t, fixture.store.providers, "company", "nothing changed")
		})
	}
}
