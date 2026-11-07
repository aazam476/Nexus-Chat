package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig/tlsconfigtest"
)

// TestHTTPSTurnsOnWithoutRestart starts the server with no certificate,
// installs one while it runs, and checks HTTPS starts working.
func TestHTTPSTurnsOnWithoutRestart(t *testing.T) {
	authority := tlsconfigtest.NewAuthority(t)
	configuration := testConfiguration(t)
	configuration.Domain = "chat.example.com"
	app, err := newApplication(t.Context(), configuration, discardLogger())
	require.NoError(t, err)
	defer app.close()
	tlsDirectory := filepath.Join(configuration.DataDirectory, "tls")
	// Trust the test CA instead of the system roots; everything else is the
	// production wiring.
	app.certificates = tlsconfig.NewManager(tlsconfig.ManagerOptions{
		Directory: tlsDirectory,
		Domain:    func(context.Context) (string, error) { return configuration.Domain, nil },
		Clock:     clock.Fixed{Time: tlsconfigtest.Now()},
		Logger:    discardLogger(),
		Roots:     authority.Roots,
	})
	app.secureServer.TLSConfig = app.certificates.TLSConfig()
	sockets := openListeners(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- app.serve(ctx, sockets) }()
	client := httpsClient(authority, tlsconfigtest.Now())
	url := fmt.Sprintf("https://%s/healthz", sockets.secure.Addr().String())

	_, err = get(t, client, url)
	require.Error(t, err, "no certificate installed yet: the handshake must fail")

	leaf, key := authority.Issue(t, tlsconfigtest.DefaultLeaf())
	require.NoError(t, os.MkdirAll(tlsDirectory, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tlsDirectory, tlsconfig.CertificateFile), authority.FullChain(leaf), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tlsDirectory, tlsconfig.KeyFile), key, 0o600))
	require.NoError(t, app.certificates.Reload(t.Context()))

	status, err := get(t, client, url)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)

	cancel()
	require.NoError(t, <-done)
}

func openListeners(t *testing.T) listeners {
	t.Helper()
	var listenConfig net.ListenConfig
	plain, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	secure, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return listeners{plain: plain, secure: secure}
}

// httpsClient trusts only the test CA and asks for chat.example.com while
// connecting to 127.0.0.1.
func httpsClient(authority tlsconfigtest.Authority, now time.Time) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			RootCAs: authority.Roots, ServerName: "chat.example.com", MinVersion: tls.VersionTLS12,
			Time: func() time.Time { return now },
		}},
	}
}

func get(t *testing.T, client *http.Client, url string) (int, error) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	_ = response.Body.Close()
	return response.StatusCode, nil
}
