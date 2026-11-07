package relayhealth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthz(t *testing.T) {
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
}

func TestOtherPathsAreNotFound(t *testing.T) {
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/push", nil))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}
