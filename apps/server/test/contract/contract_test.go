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
}

func TestHealthEndpointsMatchTheContract(t *testing.T) {
	server := startServer(t)

	for _, path := range []string{"/healthz", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			server.getAndValidate(t, path, http.StatusOK)
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
	}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
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
	return runningServer{baseURL: baseURL, router: loadContract(t)}
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

// getAndValidate calls path and checks the status code, then validates the
// request and response against the OpenAPI document.
func (server runningServer) getAndValidate(t *testing.T, path string, wantStatus int) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.baseURL+path, nil)
	require.NoError(t, err)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, wantStatus, response.StatusCode, "body: %s", body)

	// Route by path only: the contract's server URL is relative.
	routeRequest, err := http.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	require.NoError(t, err)
	route, pathParameters, err := server.router.FindRoute(routeRequest)
	require.NoError(t, err, "%s is not in the contract", path)

	input := &openapi3filter.RequestValidationInput{Request: routeRequest, PathParams: pathParameters, Route: route}
	require.NoError(t, openapi3filter.ValidateRequest(context.Background(), input))
	require.NoError(t, openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: input,
		Status:                 response.StatusCode,
		Header:                 response.Header,
		Body:                   io.NopCloser(bytes.NewReader(body)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}), "response does not match the contract: %s", body)
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
