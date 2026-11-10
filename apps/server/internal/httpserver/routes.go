package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/admin"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc"
	"github.com/aazam476/nexus-chat/apps/server/internal/config"
	"github.com/aazam476/nexus-chat/apps/server/internal/conversations"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
	"github.com/aazam476/nexus-chat/apps/server/internal/health"
	"github.com/aazam476/nexus-chat/apps/server/internal/identities"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/keybackup"
	"github.com/aazam476/nexus-chat/apps/server/internal/keypackages"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles"
	"github.com/aazam476/nexus-chat/apps/server/internal/registration"
	"github.com/aazam476/nexus-chat/apps/server/internal/setup"
	"github.com/aazam476/nexus-chat/apps/server/internal/social"
)

// Features holds the handler of every API feature.
type Features struct {
	Accounts      *accounts.Handler
	Admin         *admin.Handler
	Auth          *auth.Handler
	Conversations *conversations.Handler
	Delivery      *delivery.Handler
	Health        *health.Handler
	Identities    *identities.Handler
	Instance      *instanceconfig.Handler
	KeyBackup     *keybackup.Handler
	KeyPackages   *keypackages.Handler
	OIDC          *oidc.Handler
	Profiles      *profiles.Handler
	Registration  *registration.Handler
	Setup         *setup.Handler
	Social        *social.Handler
}

// Options configures NewHandler.
type Options struct {
	Logger         *slog.Logger
	TrustedProxies config.TrustedProxies
	// Authenticator checks the session token of protected operations.
	Authenticator Authenticator
	// Gateway serves the WebSocket at /api/v1/gateway, which is not part
	// of the OpenAPI contract (docs/realtime-protocol.md is its contract).
	Gateway http.Handler
	// WebClient serves the web app at every path the API does not use; nil
	// when the server runs without the web build.
	WebClient http.Handler
}

// Every feature's handler type is called Handler, and two embedded fields
// cannot share a name. These aliases give each one its own field name.
type (
	accountsHandler      = accounts.Handler
	adminHandler         = admin.Handler
	authHandler          = auth.Handler
	conversationsHandler = conversations.Handler
	deliveryHandler      = delivery.Handler
	healthHandler        = health.Handler
	identitiesHandler    = identities.Handler
	instanceHandler      = instanceconfig.Handler
	keyBackupHandler     = keybackup.Handler
	keyPackagesHandler   = keypackages.Handler
	oidcHandler          = oidc.Handler
	profilesHandler      = profiles.Handler
	registrationHandler  = registration.Handler
	setupHandler         = setup.Handler
	socialHandler        = social.Handler
)

// handlers combines the feature handlers into one value that implements
// api.StrictServerInterface. Each embedded handler supplies the methods for
// its own operations; the compiler reports any operation nobody implements.
type handlers struct {
	*accountsHandler
	*adminHandler
	*authHandler
	*conversationsHandler
	*deliveryHandler
	*healthHandler
	*identitiesHandler
	*instanceHandler
	*keyBackupHandler
	*keyPackagesHandler
	*oidcHandler
	*profilesHandler
	*registrationHandler
	*setupHandler
	*socialHandler
}

// Compile-time check that every OpenAPI operation has an implementation.
var _ api.StrictServerInterface = handlers{}

// NewHandler returns the server's complete HTTP handler.
func NewHandler(features Features, options Options) http.Handler {
	errorHandlers := errorResponder{logger: options.Logger}
	strict := api.NewStrictHandlerWithOptions(
		handlers{
			accountsHandler: features.Accounts, adminHandler: features.Admin, authHandler: features.Auth,
			healthHandler: features.Health, identitiesHandler: features.Identities, instanceHandler: features.Instance, keyBackupHandler: features.KeyBackup,
			keyPackagesHandler:  features.KeyPackages,
			oidcHandler:         features.OIDC,
			profilesHandler:     features.Profiles,
			registrationHandler: features.Registration, setupHandler: features.Setup,
			socialHandler: features.Social, conversationsHandler: features.Conversations, deliveryHandler: features.Delivery,
		},
		[]api.StrictMiddlewareFunc{requireSession(options.Authenticator, options.Logger)},
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  errorHandlers.badRequest,
			ResponseErrorHandlerFunc: errorHandlers.internalError,
		})
	mux := http.NewServeMux()
	if options.Gateway != nil {
		mux.Handle("GET /api/v1/gateway", options.Gateway)
	}
	if options.WebClient != nil {
		mux.Handle("GET /", options.WebClient)
	}
	routes := api.HandlerFromMux(strict, mux)
	withMiddleware := clientip.Middleware(options.TrustedProxies, securityHeaders(allowCrossOrigin(limitRequestBodies(routes))))
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
