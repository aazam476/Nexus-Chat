// Package clientip works out the real client address and scheme of each
// request (spec §3.3).
//
// Behind a reverse proxy, every connection comes from the proxy, and the
// real client is named in X-Forwarded-For / X-Forwarded-Proto. Those headers
// are trusted only when the connection comes from an address in
// TRUSTED_PROXIES; otherwise anyone could fake their IP to dodge rate limits.
//
// Start reading at [Middleware]; handlers call [FromContext].
package clientip

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/aazam476/nexus-chat/apps/server/internal/config"
)

// Info describes the client that sent a request.
type Info struct {
	// Address is the client's IP address. It is the zero Addr if the
	// connection's address could not be parsed (never in practice).
	Address netip.Addr
	// Secure reports whether the client reached us over HTTPS, either
	// directly or through a trusted proxy that terminated TLS.
	Secure bool
	// LocalHost reports whether the request was addressed to this machine
	// by a loopback name (localhost, 127.0.0.1, [::1]). Browsers treat such
	// pages as secure contexts even over plain HTTP (spec §3.3).
	LocalHost bool
}

// SecureContext reports whether a browser would allow camera, microphone,
// WebCrypto and passkeys on this connection.
func (info Info) SecureContext() bool {
	return info.Secure || info.LocalHost
}

// contextKey is unexported so no other package can collide with it.
type contextKey struct{}

// Middleware stores the request's Info in its context.
func Middleware(proxies config.TrustedProxies, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		info := resolve(request, proxies)
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), contextKey{}, info)))
	})
}

// FromContext returns the Info stored by Middleware, or the zero Info if the
// middleware did not run (for example in a unit test of a single handler).
func FromContext(ctx context.Context) Info {
	info, _ := ctx.Value(contextKey{}).(Info)
	return info
}

func resolve(request *http.Request, proxies config.TrustedProxies) Info {
	peer := peerAddress(request.RemoteAddr)
	info := Info{Address: peer, Secure: request.TLS != nil, LocalHost: isLoopbackHost(request.Host)}
	if !proxies.Contains(peer) {
		return info
	}
	if forwarded, ok := forwardedClient(request.Header.Values("X-Forwarded-For"), proxies); ok {
		info.Address = forwarded
	}
	if proto := firstValue(request.Header.Get("X-Forwarded-Proto")); proto != "" {
		info.Secure = strings.EqualFold(proto, "https")
	}
	return info
}

// forwardedClient walks X-Forwarded-For from the right (the hop nearest to
// us) and returns the first address that is not one of our proxies. Entries
// further left were written by the client itself and cannot be trusted.
func forwardedClient(headerValues []string, proxies config.TrustedProxies) (netip.Addr, bool) {
	var hops []netip.Addr
	for _, value := range headerValues {
		for _, entry := range strings.Split(value, ",") {
			address, err := netip.ParseAddr(strings.TrimSpace(entry))
			if err != nil {
				// A malformed entry breaks the chain; nothing left of it
				// can be trusted, so stop collecting.
				hops = nil
				continue
			}
			hops = append(hops, address.Unmap())
		}
	}
	for index := len(hops) - 1; index >= 0; index-- {
		if !proxies.Contains(hops[index]) {
			return hops[index], true
		}
	}
	return netip.Addr{}, false
}

func peerAddress(remoteAddress string) netip.Addr {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return address.Unmap()
}

// isLoopbackHost reports whether a Host header names this machine.
func isLoopbackHost(hostHeader string) bool {
	host, _, err := net.SplitHostPort(hostHeader)
	if err != nil {
		host = hostHeader
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}

// firstValue returns the first entry of a comma-separated header: the value
// set by the proxy closest to the client.
func firstValue(header string) string {
	first, _, _ := strings.Cut(header, ",")
	return strings.TrimSpace(first)
}
