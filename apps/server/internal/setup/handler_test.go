package setup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
)

func setupBody(token string) *api.CompleteSetupJSONRequestBody {
	domain := "chat.example.com"
	return &api.CompleteSetupJSONRequestBody{
		SetupToken: token,
		Admin:      api.SetupAdmin{Username: "admin", DisplayName: "Admin", Email: "admin@example.com", Password: "a long password"},
		Instance:   api.SetupInstance{Name: "Example", Domain: &domain, RegistrationMode: api.RegistrationModeOpen},
	}
}

// contextFor returns a context carrying clientip.Info for host, the way the
// real middleware would.
func contextFor(t *testing.T, host string) context.Context {
	t.Helper()
	var ctx context.Context
	handler := clientip.Middleware(nil, http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		ctx = request.Context()
	}))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Host = host
	handler.ServeHTTP(httptest.NewRecorder(), request)
	return ctx
}

func TestGetSetupStatus(t *testing.T) {
	service, _ := started(t, &fakeStore{}, func(d *Dependencies) { d.EnvironmentDomain = "chat.example.com" })

	response, err := NewHandler(service).GetSetupStatus(contextFor(t, "localhost:8080"), api.GetSetupStatusRequestObject{})

	require.NoError(t, err)
	domain := "chat.example.com"
	assert.Equal(t, api.GetSetupStatus200JSONResponse{
		SetupRequired: true, SecureConnection: true, Domain: &domain, DomainFromEnvironment: true,
	}, response)
}

func TestGetSetupStatusOverPlainHTTPWithoutDomain(t *testing.T) {
	service, _ := started(t, &fakeStore{}, nil)

	response, err := NewHandler(service).GetSetupStatus(contextFor(t, "192.168.1.5:8080"), api.GetSetupStatusRequestObject{})

	require.NoError(t, err)
	assert.Equal(t, api.GetSetupStatus200JSONResponse{SetupRequired: true}, response)
}

func TestGetSetupStatusPassesOnErrors(t *testing.T) {
	service := NewService(Dependencies{Store: &fakeStore{readError: errBoom}})

	_, err := NewHandler(service).GetSetupStatus(t.Context(), api.GetSetupStatusRequestObject{})

	assert.ErrorIs(t, err, errBoom)
}

func TestCompleteSetupResponses(t *testing.T) {
	tests := []struct {
		name     string
		store    *fakeStore
		change   func(*Dependencies)
		token    func(string) string
		body     func(*api.CompleteSetupJSONRequestBody)
		wantType any
		wantCode string
	}{
		{name: "success", wantType: api.CompleteSetup201JSONResponse{}},
		{name: "wrong token", token: func(string) string { return "nope" }, wantType: api.CompleteSetup401JSONResponse{}, wantCode: "invalid_setup_token"},
		{name: "already complete", store: &fakeStore{completeError: errAlreadyCompleteFromStore()}, wantType: api.CompleteSetup409JSONResponse{}, wantCode: "setup_already_complete"},
		{name: "rate limited", change: func(d *Dependencies) { d.Limiter = fakeLimiter{deny: true} }, wantType: api.CompleteSetup429JSONResponse{}, wantCode: "rate_limited"},
		{name: "invalid fields", body: func(b *api.CompleteSetupJSONRequestBody) { b.Admin.Username = "x" }, wantType: api.CompleteSetup422JSONResponse{}, wantCode: "validation_failed"},
		{name: "no domain", body: func(b *api.CompleteSetupJSONRequestBody) { b.Instance.Domain = nil }, wantType: api.CompleteSetup201JSONResponse{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := test.store
			if store == nil {
				store = &fakeStore{}
			}
			service, token := started(t, store, test.change)
			if test.token != nil {
				token = test.token(token)
			}
			body := setupBody(token)
			if test.body != nil {
				test.body(body)
			}

			response, err := NewHandler(service).CompleteSetup(contextFor(t, "localhost"), api.CompleteSetupRequestObject{Body: body})

			require.NoError(t, err)
			assert.IsType(t, test.wantType, response)
			if test.wantCode != "" {
				assert.Equal(t, test.wantCode, errorCode(response))
			}
		})
	}
}

func TestCompleteSetupValidationListsFields(t *testing.T) {
	service, token := started(t, &fakeStore{}, nil)
	body := setupBody(token)
	body.Admin.Email = "bad"

	response, err := NewHandler(service).CompleteSetup(t.Context(), api.CompleteSetupRequestObject{Body: body})

	require.NoError(t, err)
	failed, ok := response.(api.CompleteSetup422JSONResponse)
	require.True(t, ok)
	require.NotNil(t, failed.Fields)
	assert.Equal(t, []api.FieldError{{Field: "admin.email", Message: "must be a valid email address"}}, *failed.Fields)
}

func TestCompleteSetupPassesOnUnexpectedErrors(t *testing.T) {
	service, token := started(t, &fakeStore{completeError: errBoom}, nil)

	_, err := NewHandler(service).CompleteSetup(t.Context(), api.CompleteSetupRequestObject{Body: setupBody(token)})

	assert.ErrorIs(t, err, errBoom)
}

func errorCode(response api.CompleteSetupResponseObject) string {
	switch typed := response.(type) {
	case api.CompleteSetup401JSONResponse:
		return typed.Code
	case api.CompleteSetup409JSONResponse:
		return typed.Code
	case api.CompleteSetup422JSONResponse:
		return typed.Code
	case api.CompleteSetup429JSONResponse:
		return typed.Code
	}
	return ""
}
