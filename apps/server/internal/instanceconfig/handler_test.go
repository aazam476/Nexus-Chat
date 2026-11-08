package instanceconfig

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

func TestGetInstanceInfoOnAFreshInstance(t *testing.T) {
	handler := NewHandler(NewService(&fakeSettingsStore{values: map[string][]byte{}}, clock.Fixed{}, ""))

	response, err := handler.GetInstanceInfo(t.Context(), api.GetInstanceInfoRequestObject{})

	require.NoError(t, err)
	assert.Equal(t, api.GetInstanceInfo200JSONResponse{Name: "Nexus Chat", RegistrationMode: api.RegistrationModeInviteOnly}, response)
}

func TestGetInstanceInfoReportsSettingsAndSecureConnection(t *testing.T) {
	settings := &fakeSettingsStore{values: map[string][]byte{
		KeyInstanceName: []byte(`"Example"`), KeyRegistrationMode: []byte(`"open"`),
	}}
	handler := NewHandler(NewService(settings, clock.Fixed{}, "chat.example.com"))
	var response api.GetInstanceInfoResponseObject
	var err error
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://chat.example.com/api/v1/instance", nil)
	clientip.Middleware(nil, http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		response, err = handler.GetInstanceInfo(request.Context(), api.GetInstanceInfoRequestObject{})
	})).ServeHTTP(httptest.NewRecorder(), request)

	require.NoError(t, err)
	domain := "chat.example.com"
	assert.Equal(t, api.GetInstanceInfo200JSONResponse{
		Name: "Example", Domain: &domain, RegistrationMode: api.RegistrationModeOpen, SecureConnection: true,
	}, response)
}

func TestGetInstanceInfoReportsStoreFailures(t *testing.T) {
	failing := &fakeSettingsStore{err: errors.New("down")}
	_, err := NewHandler(NewService(failing, clock.Fixed{}, "")).GetInstanceInfo(t.Context(), api.GetInstanceInfoRequestObject{})
	require.ErrorContains(t, err, "down")

	// Later lookups fail too: each one is reported, not skipped.
	for _, key := range []string{KeyRegistrationMode, KeyInstanceDomain} {
		corrupt := &fakeSettingsStore{values: map[string][]byte{key: []byte(`{`)}}
		_, err := NewHandler(NewService(corrupt, clock.Fixed{}, "")).GetInstanceInfo(t.Context(), api.GetInstanceInfoRequestObject{})
		assert.ErrorContains(t, err, "decoding setting "+key)
	}
}
