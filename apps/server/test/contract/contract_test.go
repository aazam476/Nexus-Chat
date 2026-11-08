package contract

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
)

// runningServer is a server process started for one test.
type runningServer struct {
	baseURL string
	router  routers.Router
	output  *lockedBuffer
}

func TestHealthEndpointsMatchTheContract(t *testing.T) {
	server := startServer(t)

	for _, path := range []string{"/healthz", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			server.call(t, http.MethodGet, path, "", http.StatusOK)
		})
	}
}

func startServer(t *testing.T) runningServer {
	t.Helper()
	binary := buildServer(t)
	port := freePort(t)
	// Not t.Context(): that is cancelled before cleanups run, which would
	// kill the server instead of letting it shut down gracefully.
	command := exec.CommandContext(context.WithoutCancel(t.Context()), binary)
	command.Env = []string{
		"ADMIN_EMAIL=admin@example.com",
		"DATABASE_URL=" + databasetest.NewDatabaseURL(t),
		"DATA_DIR=" + t.TempDir(),
		"HTTP_PORT=" + strconv.Itoa(port),
		"HTTPS_PORT=" + strconv.Itoa(freePort(t)),
	}
	output := &lockedBuffer{}
	command.Stdout, command.Stderr = output, output
	require.NoError(t, command.Start())
	t.Cleanup(func() {
		_ = command.Process.Signal(os.Interrupt)
		_ = command.Wait()
		if t.Failed() {
			t.Logf("server output:\n%s", output.String())
		}
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitUntilListening(t, baseURL)
	return runningServer{baseURL: baseURL, router: loadContract(t), output: output}
}

func buildServer(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "nexus-chat-server")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "../../cmd/nexus-chat-server")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "building server: %s", output)
	return binary
}

func loadContract(t *testing.T) routers.Router {
	t.Helper()
	document, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))
	router, err := gorillamux.NewRouter(document)
	require.NoError(t, err)
	return router
}

// call sends a request with an optional JSON body, checks the status code,
// validates request and response against the OpenAPI document, and returns
// the response body.
func (server runningServer) call(t *testing.T, method string, path string, body string, wantStatus int) []byte { //nolint:revive // test helper; a parameter object would make every call site harder to read
	t.Helper()
	return server.callWithHeaders(t, requestSpec{method: method, path: path, body: body}, wantStatus)
}

// requestSpec describes one request made by callWithHeaders.
type requestSpec struct {
	method  string
	path    string
	body    string
	headers map[string]string
}

func (server runningServer) callWithHeaders(t *testing.T, spec requestSpec, wantStatus int) []byte {
	t.Helper()
	method, path, body := spec.method, spec.path, spec.body
	request, err := http.NewRequestWithContext(t.Context(), method, server.baseURL+path, strings.NewReader(body))
	require.NoError(t, err)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range spec.headers {
		request.Header.Set(name, value)
	}
	// Redirects are part of the contract (the provider callback), so they
	// are checked, not followed.
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, wantStatus, response.StatusCode, "body: %s", responseBody)

	server.validate(t, validationTarget{method: method, path: path, requestBody: body, headers: spec.headers, response: response, responseBody: responseBody})
	return responseBody
}

type validationTarget struct {
	method       string
	path         string
	requestBody  string
	headers      map[string]string
	response     *http.Response
	responseBody []byte
}

func (server runningServer) validate(t *testing.T, target validationTarget) {
	t.Helper()
	// Route by path only: the contract's server URL is relative.
	routeRequest, err := http.NewRequestWithContext(t.Context(), target.method, target.path, strings.NewReader(target.requestBody))
	require.NoError(t, err)
	if target.requestBody != "" {
		routeRequest.Header.Set("Content-Type", "application/json")
	}
	for name, value := range target.headers {
		routeRequest.Header.Set(name, value)
	}
	route, pathParameters, err := server.router.FindRoute(routeRequest)
	require.NoError(t, err, "%s %s is not in the contract", target.method, target.path)

	input := &openapi3filter.RequestValidationInput{Request: routeRequest, PathParams: pathParameters, Route: route}
	// Requests that are invalid on purpose (malformed bodies, missing
	// tokens) are not validated; their responses still are. Security is
	// checked by the server itself, so the validator skips it.
	input.Options = &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}
	if target.response.StatusCode != http.StatusBadRequest && target.response.StatusCode != http.StatusUnauthorized {
		require.NoError(t, openapi3filter.ValidateRequest(context.Background(), input))
	}
	require.NoError(t, openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: input,
		Status:                 target.response.StatusCode,
		Header:                 target.response.Header,
		Body:                   io.NopCloser(bytes.NewReader(target.responseBody)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}), "response does not match the contract: %s", target.responseBody)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func waitUntilListening(t *testing.T, baseURL string) {
	t.Helper()
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/healthz", nil)
		require.NoError(t, err)
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("server at %s never started", baseURL)
}

// lockedBuffer collects the server's output; the process writes from its
// own goroutines while the test reads.
type lockedBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (locked *lockedBuffer) Write(data []byte) (int, error) {
	locked.mutex.Lock()
	defer locked.mutex.Unlock()
	return locked.buffer.Write(data)
}

func (locked *lockedBuffer) String() string {
	locked.mutex.Lock()
	defer locked.mutex.Unlock()
	return locked.buffer.String()
}
