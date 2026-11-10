package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBodiesAreCutOffAtTheLimit(t *testing.T) {
	var read int
	var readErr error
	handler := limitRequestBodies(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		var body []byte
		body, readErr = io.ReadAll(request.Body)
		read = len(body)
	}))

	small := strings.NewReader(strings.Repeat("a", MaximumRequestBody))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/x", small))
	require.NoError(t, readErr)
	assert.Equal(t, MaximumRequestBody, read)

	large := strings.NewReader(strings.Repeat("a", MaximumRequestBody+1))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/x", large))
	var tooLarge *http.MaxBytesError
	require.ErrorAs(t, readErr, &tooLarge)
	assert.Equal(t, MaximumRequestBody, read)
}
