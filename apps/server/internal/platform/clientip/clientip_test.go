package clientip

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/config"
)

func capture(t *testing.T, proxies string, prepare func(*http.Request)) Info {
	t.Helper()
	trusted, err := config.ParseTrustedProxies(proxies)
	require.NoError(t, err)
	var got Info
	handler := Middleware(trusted, http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		got = FromContext(request.Context())
	}))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	prepare(request)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	return got
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name        string
		proxies     string
		remote      string
		forwarded   []string
		proto       string
		viaTLS      bool
		wantAddress string
		wantSecure  bool
	}{
		{name: "direct, no proxy configured", remote: "203.0.113.7:5000", wantAddress: "203.0.113.7"},
		{name: "direct over TLS", remote: "203.0.113.7:5000", viaTLS: true, wantAddress: "203.0.113.7", wantSecure: true},
		{name: "headers ignored from untrusted peer", proxies: "10.0.0.0/8", remote: "203.0.113.7:5000",
			forwarded: []string{"198.51.100.1"}, proto: "https", wantAddress: "203.0.113.7"},
		{name: "trusted proxy forwards client", proxies: "10.0.0.0/8", remote: "10.0.0.2:5000",
			forwarded: []string{"198.51.100.1"}, proto: "https", wantAddress: "198.51.100.1", wantSecure: true},
		{name: "spoofed leftmost entry is skipped", proxies: "10.0.0.0/8", remote: "10.0.0.2:5000",
			forwarded: []string{"1.2.3.4, 198.51.100.1"}, wantAddress: "198.51.100.1"},
		{name: "chain of trusted proxies", proxies: "10.0.0.0/8", remote: "10.0.0.2:5000",
			forwarded: []string{"198.51.100.1, 10.0.0.9", "10.0.0.8"}, wantAddress: "198.51.100.1"},
		{name: "all hops trusted keeps peer", proxies: "10.0.0.0/8", remote: "10.0.0.2:5000",
			forwarded: []string{"10.0.0.9"}, wantAddress: "10.0.0.2"},
		{name: "malformed entry breaks chain", proxies: "10.0.0.0/8", remote: "10.0.0.2:5000",
			forwarded: []string{"198.51.100.1, garbage, 10.0.0.9"}, wantAddress: "10.0.0.2"},
		{name: "forwarded http overrides TLS", proxies: "10.0.0.0/8", remote: "10.0.0.2:5000",
			proto: "http", viaTLS: true, wantAddress: "10.0.0.2"},
		{name: "proto uses first value, any case", proxies: "10.0.0.0/8", remote: "10.0.0.2:5000",
			proto: "HTTPS, http", wantAddress: "10.0.0.2", wantSecure: true},
		{name: "ipv4-mapped peer", proxies: "10.0.0.0/8", remote: "[::ffff:10.0.0.2]:5000",
			forwarded: []string{"::ffff:198.51.100.1"}, wantAddress: "198.51.100.1"},
		{name: "remote without port", remote: "203.0.113.7", wantAddress: "203.0.113.7"},
		{name: "unparsable remote", remote: "not-an-address", wantAddress: "invalid IP"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := capture(t, test.proxies, func(request *http.Request) {
				request.RemoteAddr = test.remote
				for _, value := range test.forwarded {
					request.Header.Add("X-Forwarded-For", value)
				}
				if test.proto != "" {
					request.Header.Set("X-Forwarded-Proto", test.proto)
				}
				if test.viaTLS {
					request.TLS = &tls.ConnectionState{}
				}
			})

			assert.Equal(t, test.wantAddress, info.Address.String())
			assert.Equal(t, test.wantSecure, info.Secure)
		})
	}
}

func TestFromContextWithoutMiddleware(t *testing.T) {
	assert.Equal(t, Info{}, FromContext(context.Background()))
	assert.False(t, FromContext(context.Background()).Address.IsValid())
	assert.Equal(t, netip.Addr{}, FromContext(context.Background()).Address)
}

func TestLocalHostAndSecureContext(t *testing.T) {
	tests := []struct {
		host       string
		viaTLS     bool
		wantLocal  bool
		wantSecure bool
	}{
		{host: "localhost", wantLocal: true, wantSecure: true},
		{host: "LOCALHOST:8080", wantLocal: true, wantSecure: true},
		{host: "127.0.0.1:8080", wantLocal: true, wantSecure: true},
		{host: "[::1]:8080", wantLocal: true, wantSecure: true},
		{host: "[::1]", wantLocal: true, wantSecure: true},
		{host: "chat.example.com"},
		{host: "192.168.1.20:8080"},
		{host: "chat.example.com", viaTLS: true, wantSecure: true},
	}
	for _, test := range tests {
		t.Run(test.host, func(t *testing.T) {
			info := capture(t, "", func(request *http.Request) {
				request.Host = test.host
				if test.viaTLS {
					request.TLS = &tls.ConnectionState{}
				}
			})

			assert.Equal(t, test.wantLocal, info.LocalHost)
			assert.Equal(t, test.wantSecure, info.SecureContext())
		})
	}
}
