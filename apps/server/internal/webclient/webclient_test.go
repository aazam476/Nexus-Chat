package webclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

var build = fstest.MapFS{
	"index.html":                     {Data: []byte("<!doctype html><title>Nexus Chat</title>")},
	"favicon.ico":                    {Data: []byte("icon")},
	"_expo/static/js/web/entry-1.js": {Data: []byte("console.log(1)")},
	"assets/fonts":                   {Mode: 0o755 | 1<<31},
}

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	NewHandlerFS(build).ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return recorder
}

func TestAppRoutesGetTheIndexPage(t *testing.T) {
	for _, path := range []string{"/", "/login", "/setup", "/oidc-return", "/assets/fonts"} {
		response := get(t, path)

		assert.Equal(t, http.StatusOK, response.Code, path)
		assert.Contains(t, response.Body.String(), "<title>Nexus Chat</title>", path)
		assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"), path)
		assert.Contains(t, response.Header().Get("Content-Security-Policy"), "script-src 'self'", path)
		assert.Contains(t, response.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'", path)
	}
}

func TestFilesAreServedAndHashedBundlesCachedForever(t *testing.T) {
	bundle := get(t, "/_expo/static/js/web/entry-1.js")
	assert.Equal(t, "console.log(1)", bundle.Body.String())
	assert.Contains(t, bundle.Header().Get("Cache-Control"), "immutable")

	icon := get(t, "/favicon.ico")
	assert.Equal(t, "icon", icon.Body.String())
	assert.Empty(t, icon.Header().Get("Cache-Control"))
}

func TestNewHandlerReadsADirectory(t *testing.T) {
	directory := t.TempDir()
	recorder := httptest.NewRecorder()

	NewHandler(directory).ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusNotFound, recorder.Code, "an empty build has no index page")
}

func TestPathsThatClimbOutAreRefused(t *testing.T) {
	response := get(t, "/../../etc/passwd")

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.NotContains(t, response.Body.String(), "root:")
}
