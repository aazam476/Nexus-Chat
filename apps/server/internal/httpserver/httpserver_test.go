package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/health"
)

type failingChecker struct{}

func (failingChecker) Ping(context.Context) error { return errors.New("down") }

func newTestHandler(checkers []health.NamedChecker, logs *bytes.Buffer) http.Handler {
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	features := Features{Health: health.NewHandler(health.NewService(checkers))}
	return NewHandler(features, Options{Logger: logger})
}

func TestRoutesServeHealthEndpoints(t *testing.T) {
	var logs bytes.Buffer
	handler := newTestHandler([]health.NamedChecker{{Name: "database", Checker: failingChecker{}}}, &logs)

	tests := []struct {
		path       string
		wantStatus int
		wantState  string
	}{
		{"/healthz", http.StatusOK, "ok"},
		{"/readyz", http.StatusServiceUnavailable, "unavailable"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil))

			assert.Equal(t, test.wantStatus, recorder.Code)
			var body struct{ Status string }
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			assert.Equal(t, test.wantState, body.Status)
			assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
		})
	}
}

func TestRequestsAreLoggedWithoutQueryStrings(t *testing.T) {
	var logs bytes.Buffer
	handler := newTestHandler(nil, &logs)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz?token=secret", nil))

	assert.Contains(t, logs.String(), `"path":"/healthz"`)
	assert.Contains(t, logs.String(), `"status":200`)
	assert.NotContains(t, logs.String(), "secret")
}

func TestUnknownRoutesReturnNotFound(t *testing.T) {
	var logs bytes.Buffer
	recorder := httptest.NewRecorder()

	newTestHandler(nil, &logs).ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nope", nil))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Contains(t, logs.String(), `"status":404`)
}

func TestRecoverPanicsReturns500AndLogs(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	recorder := httptest.NewRecorder()

	recoverPanics(panicking, logger).ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, logs.String(), "boom")
}

func TestRecoverPanicsLetsAbortHandlerThrough(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	aborting := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		recoverPanics(aborting, logger).ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	})
}

func TestStatusRecorderUnwraps(t *testing.T) {
	inner := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: inner}

	assert.Same(t, inner, recorder.Unwrap())
}

func TestNewServerSetsTimeouts(t *testing.T) {
	server := NewServer(":8080", http.NotFoundHandler(), slog.New(slog.DiscardHandler))

	assert.Equal(t, ":8080", server.Addr)
	assert.NotZero(t, server.ReadHeaderTimeout)
	assert.NotZero(t, server.ReadTimeout)
	assert.NotZero(t, server.WriteTimeout)
	assert.NotZero(t, server.IdleTimeout)
	assert.NotNil(t, server.ErrorLog)
}

func TestBadRequestBodiesGetJSONErrors(t *testing.T) {
	var logs bytes.Buffer
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/setup", strings.NewReader("{not json"))

	newTestHandler(nil, &logs).ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	var body struct{ Code string }
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "bad_request", body.Code)
}

func TestInternalErrorsAreLoggedButNotShown(t *testing.T) {
	var logs bytes.Buffer
	responder := errorResponder{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	recorder := httptest.NewRecorder()

	responder.internalError(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil), errors.New("database password is wrong"))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "database password")
	assert.Contains(t, recorder.Body.String(), "internal_error")
	assert.Contains(t, logs.String(), "database password is wrong")
}

func TestTheGatewayIsMountedOutsideTheContract(t *testing.T) {
	var logs bytes.Buffer
	gateway := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusTeapot) })
	handler := NewHandler(Features{}, Options{Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Gateway: gateway})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/gateway", nil))

	assert.Equal(t, http.StatusTeapot, recorder.Code, "no session is needed to open the socket; identify carries the token")
}
