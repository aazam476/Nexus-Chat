package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/config"
	"github.com/aazam476/nexus-chat/apps/server/internal/health"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
	"github.com/aazam476/nexus-chat/apps/server/internal/setup"
)

// Features holds the handler of every API feature.
type Features struct {
	Health *health.Handler
	Setup  *setup.Handler
}

// Options configures NewHandler.
type Options struct {
	Logger         *slog.Logger
	TrustedProxies config.TrustedProxies
}

// Every feature's handler type is called Handler, and two embedded fields
// cannot share a name. These aliases give each one its own field name.
type (
	healthHandler = health.Handler
	setupHandler  = setup.Handler
)

// handlers combines the feature handlers into one value that implements
// api.StrictServerInterface. Each embedded handler supplies the methods for
// its own operations; the compiler reports any operation nobody implements.
type handlers struct {
	*healthHandler
	*setupHandler
}

// Compile-time check that every OpenAPI operation has an implementation.
var _ api.StrictServerInterface = handlers{}

// NewHandler returns the server's complete HTTP handler.
func NewHandler(features Features, options Options) http.Handler {
	errorHandlers := errorResponder{logger: options.Logger}
	strict := api.NewStrictHandlerWithOptions(
		handlers{healthHandler: features.Health, setupHandler: features.Setup},
		nil,
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  errorHandlers.badRequest,
			ResponseErrorHandlerFunc: errorHandlers.internalError,
		})
	routes := api.HandlerFromMux(strict, http.NewServeMux())
	withMiddleware := clientip.Middleware(options.TrustedProxies, securityHeaders(routes))
	return recoverPanics(logRequests(withMiddleware, options.Logger), options.Logger)
}

// errorResponder writes the JSON error bodies the OpenAPI contract
// promises for malformed requests and unexpected failures.
type errorResponder struct {
	logger *slog.Logger
}

// badRequest is called when a request body cannot be decoded.
func (responder errorResponder) badRequest(writer http.ResponseWriter, _ *http.Request, err error) {
	writeError(writer, http.StatusBadRequest, api.Error{Code: "bad_request", Message: err.Error()})
}

// internalError is called when a handler returns an error. The details go
// to the log only; clients get a generic message.
func (responder errorResponder) internalError(writer http.ResponseWriter, request *http.Request, err error) {
	responder.logger.ErrorContext(request.Context(), "request failed",
		slog.String("path", request.URL.Path), slog.Any("error", err))
	writeError(writer, http.StatusInternalServerError, api.Error{Code: "internal_error", Message: "Something went wrong on the server."})
}

func writeError(writer http.ResponseWriter, status int, body api.Error) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	// The status line is already sent, so a failed write cannot be
	// reported to the client.
	_ = json.NewEncoder(writer).Encode(body)
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
