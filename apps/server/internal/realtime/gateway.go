package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
)

// Subprotocol is the WebSocket subprotocol of protocol version 1.
const Subprotocol = "nexus-chat.v1"

// Timing defaults (docs/realtime-protocol.md §3).
const (
	defaultHeartbeatInterval = 30 * time.Second
	defaultIdentifyTimeout   = 10 * time.Second
	writeTimeout             = 10 * time.Second
)

var errBadToken = errors.New("the token is invalid, expired or revoked")

type authenticator interface {
	Authenticate(ctx context.Context, token string) (sessionstore.Session, error)
}

type userLoader interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (store.User, error)
}

// GatewayOptions configure NewGateway. Zero durations use the protocol's
// defaults; tests shorten them.
type GatewayOptions struct {
	Hub               *Hub
	Sessions          authenticator
	Users             userLoader
	Logger            *slog.Logger
	HeartbeatInterval time.Duration
	IdentifyTimeout   time.Duration
}

// Gateway accepts WebSocket connections at /api/v1/gateway.
type Gateway struct {
	options GatewayOptions
	// closing is cancelled by Close. Hijacked connections are not closed
	// by http.Server.Shutdown, so every connection watches it.
	closing context.Context
	close   context.CancelFunc
}

// NewGateway returns a Gateway.
func NewGateway(options GatewayOptions) *Gateway {
	if options.HeartbeatInterval == 0 {
		options.HeartbeatInterval = defaultHeartbeatInterval
	}
	if options.IdentifyTimeout == 0 {
		options.IdentifyTimeout = defaultIdentifyTimeout
	}
	closing, cancel := context.WithCancel(context.Background())
	return &Gateway{options: options, closing: closing, close: cancel}
}

// Close disconnects every connection; devices resume once the server is
// back. New connections are refused.
func (gateway *Gateway) Close() {
	gateway.close()
}

// ServeHTTP upgrades the request and serves the connection until it ends.
func (gateway *Gateway) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if gateway.closing.Err() != nil {
		http.Error(writer, "the server is shutting down", http.StatusServiceUnavailable)
		return
	}
	// The server's request timeouts would otherwise end a long-lived
	// WebSocket after a minute. Heartbeats take their place.
	controller := http.NewResponseController(writer)
	_ = controller.SetReadDeadline(time.Time{})  // only unsupported in tests' recorders
	_ = controller.SetWriteDeadline(time.Time{}) // same
	conn, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		Subprotocols: []string{Subprotocol},
		// Any origin may connect: the gateway never uses cookies, so a page
		// on another site has no credential to send (protocol §1).
		InsecureSkipVerify: true,
	})
	if err != nil {
		return // Accept has already answered the request
	}
	if conn.Subprotocol() != Subprotocol {
		_ = conn.Close(websocket.StatusPolicyViolation, "use the "+Subprotocol+" subprotocol")
		return
	}
	conn.SetReadLimit(maximumFrameBytes)
	newConnection(gateway, conn).run(request.Context())
}

// authenticate checks a token and loads its account.
func (gateway *Gateway) authenticate(ctx context.Context, token string) (store.User, error) {
	found, err := gateway.options.Sessions.Authenticate(ctx, token)
	if errors.Is(err, session.ErrInvalidSession) {
		return store.User{}, errBadToken
	}
	if err != nil {
		return store.User{}, err
	}
	user, err := gateway.options.Users.GetUserByID(ctx, found.UserID)
	if err != nil {
		return store.User{}, fmt.Errorf("loading account: %w", err)
	}
	return user, nil
}
