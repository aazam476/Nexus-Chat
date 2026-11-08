package realtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/time/rate"
)

// Close codes (docs/realtime-protocol.md §7).
const (
	closeMalformed     websocket.StatusCode = 4000
	closeNotIdentified websocket.StatusCode = 4003
	closeBadToken      websocket.StatusCode = 4004
	closeAlreadyReady  websocket.StatusCode = 4005
	closeRateLimited   websocket.StatusCode = 4008
	closeTimedOut      websocket.StatusCode = 4009
	closeReplaced      websocket.StatusCode = 4010
)

// Frame rate limit: 30 frames per 10 seconds (protocol §2).
const (
	framesPerWindow = 30
	frameWindow     = 10 * time.Second
	// handshakeFrames bounds resume attempts before identify.
	handshakeFrames = 3
)

// closeReason is why the server ends a connection.
type closeReason struct {
	code websocket.StatusCode
	text string
}

// normalClose ends a connection without an error.
func normalClose() *closeReason {
	return &closeReason{websocket.StatusNormalClosure, ""}
}

// serverError ends a connection after an unexpected failure.
func serverError() *closeReason {
	return &closeReason{websocket.StatusInternalError, "internal error"}
}

// connection is one WebSocket.
type connection struct {
	gateway *Gateway
	conn    *websocket.Conn
	outbox  chan []byte
	// closing makes sure the connection is closed once, with the first
	// reason given; later reasons are ignored.
	closing sync.Once
}

func newConnection(gateway *Gateway, conn *websocket.Conn) *connection {
	return &connection{gateway: gateway, conn: conn, outbox: make(chan []byte, outboxSize)}
}

// run speaks the protocol until the connection ends.
func (connection *connection) run(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	// Closing the connection (not cancelling ctx) when the gateway closes:
	// a cancelled read would drop the connection without a close frame.
	stop := context.AfterFunc(connection.gateway.closing, func() { //nolint:contextcheck // see above
		connection.closeLater(&closeReason{websocket.StatusGoingAway, "the server is restarting; resume"})
	})
	defer stop()
	connection.close(connection.serve(ctx))
}

// close sends a close frame with reason and ends the connection. It may
// block for a few seconds while the peer answers.
func (connection *connection) close(reason *closeReason) {
	connection.closing.Do(func() {
		_ = connection.conn.Close(reason.code, reason.text) // the peer may already be gone
	})
}

// closeLater closes without blocking the caller, which may hold locks or
// be a timer. The goroutine ends when Close returns.
func (connection *connection) closeLater(reason *closeReason) {
	go connection.close(reason)
}

func (connection *connection) serve(ctx context.Context) *closeReason {
	hello := helloData{HeartbeatIntervalMs: connection.gateway.options.HeartbeatInterval.Milliseconds(), ProtocolVersion: 1}
	if err := connection.write(ctx, encode(serverFrame{Type: typeHello, Data: hello})); err != nil {
		return normalClose()
	}
	session, token, reason := connection.handshake(ctx)
	if reason != nil {
		return reason
	}
	defer connection.gateway.options.Hub.disconnect(session, connection.outbox)
	// The writer stops when ctx is cancelled: when this function returns,
	// the connection is kicked, or the gateway closes.
	go connection.writeLoop(ctx)
	return connection.readLoop(ctx, session, token)
}

// kick ends the connection from the hub's side (resumed elsewhere, or too
// slow). It must not block: the hub holds its locks while calling it.
func (connection *connection) kick() {
	connection.closeLater(&closeReason{closeReplaced, "the session continues on another connection; resume to continue here"})
}

// writeLoop sends queued frames in order.
func (connection *connection) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-connection.outbox:
			if err := connection.write(ctx, frame); err != nil {
				return // the connection is closing; the reader sees it too
			}
		}
	}
}

func (connection *connection) write(ctx context.Context, frame []byte) error {
	writeContext, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if err := connection.conn.Write(writeContext, websocket.MessageText, frame); err != nil {
		return fmt.Errorf("writing frame: %w", err)
	}
	return nil
}

// send queues a frame that is not an event, behind any queued events.
func (connection *connection) send(ctx context.Context, frame serverFrame) {
	select {
	case connection.outbox <- encode(frame):
	case <-ctx.Done():
	}
}

// read returns the next frame, or why the connection must close. If
// nothing arrives within timeout, the connection is closed with
// timeoutReason; a timer does this because a read whose context expires
// would drop the connection without telling the peer why.
func (connection *connection) read(ctx context.Context, timeout time.Duration, timeoutReason *closeReason) (clientFrame, *closeReason) {
	timer := time.AfterFunc(timeout, func() { connection.closeLater(timeoutReason) })
	defer timer.Stop()
	messageType, message, err := connection.conn.Read(ctx)
	switch {
	case err != nil:
		return clientFrame{}, normalClose() // closed by the peer, a timer, a kick or shutdown
	case messageType != websocket.MessageText:
		return clientFrame{}, &closeReason{closeMalformed, "frames must be JSON text"}
	}
	frame, err := decodeClientFrame(message)
	if err != nil {
		return clientFrame{}, &closeReason{closeMalformed, "malformed frame"}
	}
	return frame, nil
}

func (connection *connection) link() link {
	return link{outbox: connection.outbox, kick: connection.kick}
}

func (connection *connection) logger() *slog.Logger {
	return connection.gateway.options.Logger
}

func newFrameLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Every(frameWindow/framesPerWindow), framesPerWindow)
}
