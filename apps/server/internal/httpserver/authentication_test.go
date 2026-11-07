package httpserver

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
)

// TestPublicOperationsMatchTheContract keeps publicOperations in step with
// api/openapi.yaml: an operation is public exactly when the contract
// declares no security requirement for it.
func TestPublicOperationsMatchTheContract(t *testing.T) {
	document, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	contractPublic := map[string]bool{}
	for _, item := range document.Paths.Map() {
		for _, operation := range item.Operations() {
			if operation.Security == nil || len(*operation.Security) == 0 {
				contractPublic[goName(operation.OperationID)] = true
			}
		}
	}

	assert.Equal(t, contractPublic, publicOperations)
}

// goName turns an operationId such as "getLiveness" into the Go name
// oapi-codegen gives it ("GetLiveness").
func goName(operationID string) string {
	runes := []rune(operationID)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

type fakeAuthenticator struct {
	session sessionstore.Session
	err     error
}

func (fake fakeAuthenticator) Authenticate(_ context.Context, token string) (sessionstore.Session, error) {
	if fake.err != nil {
		return sessionstore.Session{}, fake.err
	}
	if token != "nxs_good" {
		return sessionstore.Session{}, session.ErrInvalidSession
	}
	return fake.session, nil
}

func runProtected(t *testing.T, authenticator Authenticator, header string) (*httptest.ResponseRecorder, *session.Principal, error) {
	t.Helper()
	var seen *session.Principal
	next := func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		if principal, ok := session.PrincipalFrom(ctx); ok {
			seen = &principal
		}
		return "handled", nil
	}
	wrapped := requireSession(authenticator, slog.New(slog.DiscardHandler))(next, "ListSessions")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/sessions", nil)
	if header != "" {
		request.Header.Set("Authorization", header)
	}
	_, err := wrapped(request.Context(), recorder, request, nil)
	return recorder, seen, err
}

func TestProtectedOperationsNeedAValidSession(t *testing.T) {
	valid := sessionstore.Session{ID: uuid.New(), UserID: uuid.New()}
	authenticator := fakeAuthenticator{session: valid}

	for name, header := range map[string]string{"no header": "", "wrong scheme": "Basic abc", "unknown token": "Bearer nxs_bad"} {
		t.Run(name, func(t *testing.T) {
			recorder, seen, err := runProtected(t, authenticator, header)

			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"code":"unauthorized"`)
			assert.Equal(t, `Bearer realm="nexus-chat"`, recorder.Header().Get("WWW-Authenticate"))
			assert.Nil(t, seen, "the handler must not run")
		})
	}

	_, seen, err := runProtected(t, authenticator, "Bearer nxs_good")
	require.NoError(t, err)
	require.NotNil(t, seen)
	assert.Equal(t, session.Principal{UserID: valid.UserID, SessionID: valid.ID}, *seen)
}

func TestAuthenticatorFailuresBecomeErrors(t *testing.T) {
	failure := errors.New("database down")

	_, seen, err := runProtected(t, fakeAuthenticator{err: failure}, "Bearer nxs_good")

	require.ErrorIs(t, err, failure)
	assert.Nil(t, seen)
}

func TestPublicOperationsSkipAuthentication(t *testing.T) {
	called := false
	next := func(context.Context, http.ResponseWriter, *http.Request, any) (any, error) {
		called = true
		return nil, nil
	}

	wrapped := requireSession(nil, slog.New(slog.DiscardHandler))(next, "Login")
	_, err := wrapped(t.Context(), httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), nil)

	require.NoError(t, err)
	assert.True(t, called)
}

func TestProtectedRouteThroughTheFullHandler(t *testing.T) {
	var logs bytes.Buffer
	handler := NewHandler(Features{}, Options{Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Authenticator: fakeAuthenticator{}})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/users/me", nil))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.True(t, strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json"))
}
