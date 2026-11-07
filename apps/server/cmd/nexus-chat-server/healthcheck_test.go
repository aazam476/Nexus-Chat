package main

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serverOnPort(t *testing.T, status int) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	return port
}

func TestHealthcheck(t *testing.T) {
	tests := map[string]struct {
		port     string
		wantCode int
		wantText string
	}{
		"healthy":       {port: serverOnPort(t, http.StatusOK), wantCode: 0},
		"unhealthy":     {port: serverOnPort(t, http.StatusServiceUnavailable), wantCode: 1, wantText: "status 503"},
		"not listening": {port: strconv.Itoa(freePort(t)), wantCode: 1, wantText: "healthcheck:"},
		"bad port":      {port: "eighty", wantCode: 1, wantText: "not a number"},
		"bad url":       {port: "99999999999999999999", wantCode: 1, wantText: "healthcheck:"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var errorOutput bytes.Buffer

			code := runHealthcheck(t.Context(), testEnvironment(map[string]string{"HTTP_PORT": test.port}), &errorOutput)

			assert.Equal(t, test.wantCode, code)
			assert.Contains(t, errorOutput.String(), test.wantText)
		})
	}
}

func TestRunRejectsInvalidPrivilegeSetting(t *testing.T) {
	getenv := testEnvironment(map[string]string{
		"ADMIN_EMAIL": "admin@example.com", "POSTGRES_PASSWORD": "x", "DATA_DIR": t.TempDir(),
		"NEXUS_DROP_PRIVILEGES": "root",
	})

	err := run(t.Context(), getenv, &lockedBuffer{})

	assert.ErrorContains(t, err, "NEXUS_DROP_PRIVILEGES")
}
