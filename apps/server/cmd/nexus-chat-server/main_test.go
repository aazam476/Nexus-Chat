package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig"
)

// lockedBuffer is a bytes.Buffer safe to read while the server writes logs
// from another goroutine.
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

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func testEnvironment(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestRunServesHealthAndShutsDownGracefully(t *testing.T) {
	dataDirectory := t.TempDir()
	port := freePort(t)
	getenv := testEnvironment(map[string]string{
		"ADMIN_EMAIL":  "admin@example.com",
		"DATABASE_URL": databasetest.NewDatabaseURL(t),
		"DATA_DIR":     dataDirectory,
		"HTTP_PORT":    strconv.Itoa(port),
		"HTTPS_PORT":   strconv.Itoa(freePort(t)),
	})
	ctx, cancel := context.WithCancel(context.Background())
	logs := &lockedBuffer{}
	result := make(chan error, 1)
	go func() { result <- run(ctx, getenv, logs) }()

	waitForStatus(t, fmt.Sprintf("http://127.0.0.1:%d/readyz", port), http.StatusOK)
	cancel()

	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(shutdownTimeout + 2*time.Second):
		t.Fatal("server did not shut down")
	}
	assert.Contains(t, logs.String(), "applied database migration")
	assert.Contains(t, logs.String(), "shutting down")
	assert.FileExists(t, filepath.Join(dataDirectory, "secrets", federationKeyFileForTest))
}

// federationKeyFileForTest mirrors the secrets package's file name. If the
// name changes there, this test fails, which is intended: changing it would
// make upgraded instances lose their federation identity.
const federationKeyFileForTest = "federation.ed25519.pem"

func TestRunReportsConfigurationErrors(t *testing.T) {
	err := run(context.Background(), testEnvironment(nil), &lockedBuffer{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ADMIN_EMAIL is required")
}

func TestRunFailsWhenSecretsCannotBeStored(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	getenv := testEnvironment(map[string]string{
		"ADMIN_EMAIL":       "admin@example.com",
		"POSTGRES_PASSWORD": "unused",
		"DATA_DIR":          blocker,
	})

	err := run(context.Background(), getenv, &lockedBuffer{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading instance secrets")
}

func TestRunFailsWhenDatabaseIsUnreachable(t *testing.T) {
	getenv := testEnvironment(map[string]string{
		"ADMIN_EMAIL":  "admin@example.com",
		"DATABASE_URL": fmt.Sprintf("postgres://user:pass@127.0.0.1:%d/db?connect_timeout=2", freePort(t)),
		"DATA_DIR":     t.TempDir(),
	})

	err := run(context.Background(), getenv, &lockedBuffer{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connecting to database")
}

func TestRunFailsWhenPortIsTaken(t *testing.T) {
	occupied, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = occupied.Close() }()
	getenv := testEnvironment(map[string]string{
		"ADMIN_EMAIL":  "admin@example.com",
		"DATABASE_URL": databasetest.NewDatabaseURL(t),
		"DATA_DIR":     t.TempDir(),
		"HTTP_PORT":    strconv.Itoa(occupied.Addr().(*net.TCPAddr).Port),
		"HTTPS_PORT":   strconv.Itoa(freePort(t)),
	})

	err = run(context.Background(), getenv, &lockedBuffer{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listening on")
}

func TestServeReturnsListenerErrors(t *testing.T) {
	ctx := context.Background()
	app, err := newApplication(ctx, testConfiguration(t), discardLogger())
	require.NoError(t, err)
	defer app.close()
	closed, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, closed.Close()) // Serve fails immediately on a closed listener.
	working, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	err = app.serve(ctx, listeners{plain: closed, secure: working})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "serving HTTP")
}

func waitForStatus(t *testing.T, url string, want int) {
	t.Helper()
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
		require.NoError(t, err)
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == want {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s never returned %d", url, want)
}

func TestRunMainReturnsExitCodeOneAndExplains(t *testing.T) {
	var errorOutput bytes.Buffer

	code := runMain(testEnvironment(nil), io.Discard, &errorOutput)

	assert.Equal(t, 1, code)
	assert.Contains(t, errorOutput.String(), "nexus-chat-server: invalid configuration")
}

func TestRunMainStopsOnSignal(t *testing.T) {
	port := freePort(t)
	getenv := testEnvironment(map[string]string{
		"ADMIN_EMAIL":  "admin@example.com",
		"DATABASE_URL": databasetest.NewDatabaseURL(t),
		"DATA_DIR":     t.TempDir(),
		"HTTP_PORT":    strconv.Itoa(port),
		"HTTPS_PORT":   strconv.Itoa(freePort(t)),
	})
	result := make(chan int, 1)
	go func() { result <- runMain(getenv, io.Discard, io.Discard) }()
	waitForStatus(t, fmt.Sprintf("http://127.0.0.1:%d/healthz", port), http.StatusOK)

	// signal.NotifyContext intercepts the signal, so the test process is
	// not terminated; runMain sees its context cancelled instead.
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))

	select {
	case code := <-result:
		assert.Equal(t, 0, code)
	case <-time.After(shutdownTimeout + 2*time.Second):
		t.Fatal("server did not stop on SIGTERM")
	}
}

func TestRunFailsWhenHTTPSPortIsTaken(t *testing.T) {
	occupied, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = occupied.Close() }()
	getenv := testEnvironment(map[string]string{
		"ADMIN_EMAIL":  "admin@example.com",
		"DATABASE_URL": databasetest.NewDatabaseURL(t),
		"DATA_DIR":     t.TempDir(),
		"HTTP_PORT":    strconv.Itoa(freePort(t)),
		"HTTPS_PORT":   strconv.Itoa(occupied.Addr().(*net.TCPAddr).Port),
	})

	err = run(context.Background(), getenv, &lockedBuffer{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listening on")
}

func TestBadCertificateAtStartupKeepsHTTPRunning(t *testing.T) {
	configuration := testConfiguration(t)
	configuration.Domain = "chat.example.com"
	tlsDirectory := filepath.Join(configuration.DataDirectory, "tls")
	require.NoError(t, os.MkdirAll(tlsDirectory, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tlsDirectory, tlsconfig.CertificateFile), []byte("bad"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tlsDirectory, tlsconfig.KeyFile), []byte("bad"), 0o600))
	logs := &lockedBuffer{}

	app, err := newApplication(t.Context(), configuration, slog.New(slog.NewJSONHandler(logs, nil)))

	require.NoError(t, err)
	defer app.close()
	assert.False(t, app.certificates.Active())
	assert.Contains(t, logs.String(), "HTTPS is off")
}
