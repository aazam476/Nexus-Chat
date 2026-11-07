// Package relayhealth serves the relay's liveness probe at /healthz.
//
// The relay has no OpenAPI contract yet; its API (docs/push-notifications.md
// §2) is specified and generated in milestone M8.
package relayhealth

import (
	"net/http"
)

// NewHandler returns an http.Handler that serves GET /healthz.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		// A write error means the client went away; there is nobody to
		// report it to.
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}
