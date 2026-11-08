package httpserver

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCrossOriginRequestsGetNoCredentials(t *testing.T) {
	handler := allowCrossOrigin(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusTeapot) }))
	request := func(method string, path string, headers map[string]string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		incoming := httptest.NewRequestWithContext(t.Context(), method, path, nil)
		for name, value := range headers {
			incoming.Header.Set(name, value)
		}
		handler.ServeHTTP(recorder, incoming)
		return recorder
	}

	preflight := request(http.MethodOptions, "/api/v1/users/me", map[string]string{"Origin": "app://nexus-chat", "Access-Control-Request-Method": "GET"})
	assert.Equal(t, http.StatusNoContent, preflight.Code)
	assert.Equal(t, "app://nexus-chat", preflight.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, preflight.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Empty(t, preflight.Header().Get("Access-Control-Allow-Credentials"), "cookies are never allowed")

	call := request(http.MethodGet, "/api/v1/instance", map[string]string{"Origin": "app://nexus-chat"})
	assert.Equal(t, http.StatusTeapot, call.Code)
	assert.Equal(t, "app://nexus-chat", call.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Retry-After", call.Header().Get("Access-Control-Expose-Headers"))

	sameOrigin := request(http.MethodGet, "/api/v1/instance", nil)
	assert.Empty(t, sameOrigin.Header().Get("Access-Control-Allow-Origin"))
	page := request(http.MethodGet, "/login", map[string]string{"Origin": "https://evil.example"})
	assert.Empty(t, page.Header().Get("Access-Control-Allow-Origin"), "only the API is shared")
	plainOptions := request(http.MethodOptions, "/api/v1/instance", map[string]string{"Origin": "app://nexus-chat"})
	assert.Equal(t, http.StatusTeapot, plainOptions.Code, "not a preflight")
}

func TestTheWebClientServesEverythingElse(t *testing.T) {
	web := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte("app")) })
	handler := NewHandler(Features{}, Options{Logger: slog.New(slog.DiscardHandler), WebClient: web})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil))

	assert.Equal(t, "app", recorder.Body.String())
}
