package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func environment(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func TestRunServesHealthzAndStops(t *testing.T) {
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- run(ctx, environment(map[string]string{"PORT": strconv.Itoa(port)}), io.Discard) }()

	waitForHealthz(t, fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	cancel()

	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(shutdownTimeout + time.Second):
		t.Fatal("relay did not stop")
	}
}

func TestRunRejectsInvalidConfiguration(t *testing.T) {
	err := run(context.Background(), environment(map[string]string{"PORT": "x"}), io.Discard)

	assert.ErrorContains(t, err, "PORT")
}

func TestRunFailsWhenPortIsTaken(t *testing.T) {
	occupied, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = occupied.Close() }()
	port := occupied.Addr().(*net.TCPAddr).Port

	err = run(context.Background(), environment(map[string]string{"PORT": strconv.Itoa(port)}), io.Discard)

	assert.ErrorContains(t, err, "listening on port")
}

func TestServeReturnsListenerErrors(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())

	err = serve(context.Background(), &http.Server{ReadHeaderTimeout: time.Second}, listener)

	assert.ErrorContains(t, err, "serving HTTP")
}

func waitForHealthz(t *testing.T, url string) {
	t.Helper()
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
		require.NoError(t, err)
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s never became healthy", url)
}

func TestRunMainReturnsExitCodeOneAndExplains(t *testing.T) {
	var errorOutput bytes.Buffer

	code := runMain(environment(map[string]string{"PORT": "x"}), io.Discard, &errorOutput)

	assert.Equal(t, 1, code)
	assert.Contains(t, errorOutput.String(), "nexus-chat-push-relay: invalid configuration")
}

func TestRunMainStopsOnSignal(t *testing.T) {
	port := freePort(t)
	result := make(chan int, 1)
	go func() {
		result <- runMain(environment(map[string]string{"PORT": strconv.Itoa(port)}), io.Discard, io.Discard)
	}()
	waitForHealthz(t, fmt.Sprintf("http://127.0.0.1:%d/healthz", port))

	// signal.NotifyContext intercepts the signal, so the test process is
	// not terminated; runMain sees its context cancelled instead.
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))

	select {
	case code := <-result:
		assert.Equal(t, 0, code)
	case <-time.After(shutdownTimeout + time.Second):
		t.Fatal("relay did not stop on SIGTERM")
	}
}
