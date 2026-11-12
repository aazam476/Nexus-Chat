package linkpreview

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedSwitch reports the proxy on or off, or fails.
type fixedSwitch struct {
	on  bool
	err error
}

func (fixed fixedSwitch) LinkPreviewProxy(context.Context) (bool, error) { return fixed.on, fixed.err }

type limiter struct{ allow bool }

func (fake limiter) Allow(string) bool { return fake.allow }

// localService may reach the test server on 127.0.0.1, unlike a real one.
func localService(on bool) *Service {
	return NewService(Options{Switch: fixedSwitch{on: on}, Limiter: limiter{true}, Allow: func(netip.Addr) bool { return true }})
}

const page = `<!doctype html><html><head>
<title>Fallback title</title>
<meta property="og:title" content="  Nexus Chat  ">
<meta property="og:title" content="ignored second">
<meta name="description" content="Encrypted chat you host yourself.">
<meta property="og:image" content="/logo.png">
</head><body><meta property="og:description" content="too late"></body></html>`

func site(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(writer, page)
	})
	mux.HandleFunc("/bare", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(writer, `<html><head></head><body>nothing</body></html>`)
	})
	mux.HandleFunc("/titled", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(writer, `<title>Only `+strings.Repeat("x", 400)+`</title><meta name="twitter:image" content="javascript:alert(1)">`)
	})
	mux.HandleFunc("/moved", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/", http.StatusFound)
	})
	mux.HandleFunc("/loop", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/loop", http.StatusFound)
	})
	mux.HandleFunc("/away", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "ftp://example.com/x", http.StatusFound)
	})
	mux.HandleFunc("/logo.png", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write([]byte("png bytes"))
	})
	mux.HandleFunc("/huge.png", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(make([]byte, maximumImageBytes+1))
	})
	mux.HandleFunc("/text", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(writer, "plain")
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestPreviewsReadTheHeadOfAPage(t *testing.T) {
	server := site(t)
	service := localService(true)

	preview, err := service.Preview(t.Context(), "ada", server.URL+"/moved")
	require.NoError(t, err)
	assert.Equal(t, Page{Title: "Nexus Chat", Description: "Encrypted chat you host yourself.", ImageURL: server.URL + "/logo.png"}, preview)

	titled, err := service.Preview(t.Context(), "ada", server.URL+"/titled")
	require.NoError(t, err)
	assert.Len(t, []rune(titled.Title), maximumTitle)
	assert.True(t, strings.HasSuffix(titled.Title, "…"))
	assert.Empty(t, titled.ImageURL, "only http and https images")

	for _, path := range []string{"/bare", "/text", "/missing", "/loop", "/away"} {
		_, err := service.Preview(t.Context(), "ada", server.URL+path)
		require.ErrorIs(t, err, ErrUnavailable, path)
	}
}

func TestPreviewImages(t *testing.T) {
	server := site(t)
	service := localService(true)

	image, err := service.Image(t.Context(), "ada", server.URL+"/logo.png")
	require.NoError(t, err)
	assert.Equal(t, Image{Data: []byte("png bytes"), ContentType: "image/png"}, image)
	for _, path := range []string{"/huge.png", "/", "/missing"} {
		_, err := service.Image(t.Context(), "ada", server.URL+path)
		require.ErrorIs(t, err, ErrUnavailable, path)
	}
}

func TestTheProxyIsGuarded(t *testing.T) {
	server := site(t)

	_, err := localService(false).Preview(t.Context(), "ada", server.URL)
	require.ErrorIs(t, err, ErrDisabled)
	for _, link := range []string{"ftp://example.com/", "https://user:pass@example.com/", "not a link%", "https:///nohost"} {
		_, err := localService(true).Preview(t.Context(), "ada", link)
		require.ErrorIs(t, err, ErrInvalidLink, link)
	}
	limited := NewService(Options{Switch: fixedSwitch{on: true}, Limiter: limiter{false}})
	_, err = limited.Image(t.Context(), "ada", server.URL)
	require.ErrorIs(t, err, ErrRateLimited)
	failing := NewService(Options{Switch: fixedSwitch{err: errors.New("down")}, Limiter: limiter{true}})
	_, err = failing.Preview(t.Context(), "ada", server.URL)
	require.ErrorContains(t, err, "down")

	// A real service refuses the test server's loopback address.
	public := NewService(Options{Switch: fixedSwitch{on: true}, Limiter: limiter{true}})
	_, err = public.Preview(t.Context(), "ada", server.URL)
	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorContains(t, err, errForbiddenAddress.Error())
}

func TestOnlyPublicAddressesAreReached(t *testing.T) {
	for address, public := range map[string]bool{
		"93.184.216.34": true, "2606:2800:220:1::1": true,
		"127.0.0.1": false, "10.1.2.3": false, "172.16.0.1": false, "192.168.1.1": false, "169.254.169.254": false,
		"100.64.0.1": false, "0.0.0.0": false, "::1": false, "fc00::1": false, "fe80::1": false, "224.0.0.1": false,
		"::ffff:127.0.0.1": false,
	} {
		assert.Equal(t, public, publicAddress(netip.MustParseAddr(address)), address)
	}
	control := guardedDialer(publicAddress).Control
	require.ErrorIs(t, control("tcp", "127.0.0.1:80", nil), errForbiddenAddress)
	require.ErrorIs(t, control("tcp", "not-an-ip:80", nil), errForbiddenAddress)
	require.Error(t, control("tcp", "missing-port", nil))
	require.NoError(t, control("tcp", "93.184.216.34:443", nil))
}
