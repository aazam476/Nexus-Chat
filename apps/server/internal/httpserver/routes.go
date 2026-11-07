package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/health"
)

// Features holds the handler of every API feature.
type Features struct {
	Health *health.Handler
}

// handlers combines the feature handlers into one value that implements
// api.StrictServerInterface. Each embedded handler supplies the methods for
// its own operations; the compiler reports any operation nobody implements.
type handlers struct {
	*health.Handler
}

// Compile-time check that every OpenAPI operation has an implementation.
var _ api.StrictServerInterface = handlers{}

// NewHandler returns the server's complete HTTP handler.
func NewHandler(features Features, logger *slog.Logger) http.Handler {
	strict := api.NewStrictHandler(handlers{Handler: features.Health}, nil)
	mux := http.NewServeMux()
	routes := api.HandlerFromMux(strict, mux)
	return recoverPanics(logRequests(securityHeaders(routes), logger), logger)
}

// NewServer returns an *http.Server with timeouts suitable for the public
// internet. Without them, slow clients could hold connections open forever.
func NewServer(address string, handler http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
}
