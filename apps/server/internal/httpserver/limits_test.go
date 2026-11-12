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

func TestAttachmentTransfersAreLimitedByTheirOwnFeature(t *testing.T) {
	var read int
	handler := limitRequestBodies(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		read = len(body)
	}))
	large := strings.Repeat("a", MaximumRequestBody+1)

	for _, path := range []string{"/api/v1/conversations/c1/attachments", "/api/v1/conversations/c1/attachments/a1"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(large)))
		assert.Equal(t, MaximumRequestBody+1, read, path)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/conversations/c1/attachments/a1/x", strings.NewReader(large)))
	assert.Equal(t, MaximumRequestBody, read, "anything else keeps the usual limit")
}
