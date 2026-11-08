package realtime

import (
	"context"
	"errors"
	"log/slog"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
)

func notIdentified() *closeReason {
	return &closeReason{closeNotIdentified, "send identify or resume first"}
}

// handshake waits for identify or resume and attaches the gateway session.
// After a failed resume the client may identify on the same connection.
func (connection *connection) handshake(ctx context.Context) (*gatewaySession, string, *closeReason) {
	for range handshakeFrames {
		frame, reason := connection.read(ctx, connection.gateway.options.IdentifyTimeout, notIdentified())
		if reason != nil {
			return nil, "", reason
		}
		switch frame.Type {
		case typeIdentify:
			return connection.identify(ctx, frame)
		case typeResume:
			session, token, reason, retry := connection.resume(ctx, frame)
			if !retry {
				return session, token, reason
			}
			connection.writeDirect(ctx, serverFrame{Type: typeInvalidSession})
		default:
			return nil, "", notIdentified()
		}
	}
	return nil, "", notIdentified()
}

func (connection *connection) identify(ctx context.Context, frame clientFrame) (*gatewaySession, string, *closeReason) {
	var data identifyData
	if err := decodeData(frame, &data); err != nil {
		return nil, "", &closeReason{closeMalformed, "malformed identify"}
	}
	user, reason := connection.authenticate(ctx, data.Token)
	if reason != nil {
		return nil, "", reason
	}
	return connection.gateway.options.Hub.connect(user, connection.link()), data.Token, nil
}

// resume returns retry true when the session cannot be resumed and the
// client should identify instead.
func (connection *connection) resume(ctx context.Context, frame clientFrame) (*gatewaySession, string, *closeReason, bool) {
	var data resumeData
	if err := decodeData(frame, &data); err != nil {
		return nil, "", &closeReason{closeMalformed, "malformed resume"}, false
	}
	user, reason := connection.authenticate(ctx, data.Token)
	if reason != nil {
		return nil, "", reason, false
	}
	session, replayed, ok := connection.gateway.options.Hub.resume(user.ID, data.GatewaySessionID, data.LastSequence, connection.link())
	if !ok {
		return nil, "", nil, true
	}
	connection.send(ctx, serverFrame{Type: typeResumed, Data: resumedData{Replayed: replayed}})
	return session, data.Token, nil, false
}

func (connection *connection) authenticate(ctx context.Context, token string) (store.User, *closeReason) {
	user, err := connection.gateway.authenticate(ctx, token)
	if errors.Is(err, errBadToken) {
		return store.User{}, &closeReason{closeBadToken, err.Error()}
	}
	if err != nil {
		connection.logger().ErrorContext(ctx, "gateway authentication failed", slog.Any("error", err))
		return store.User{}, serverError()
	}
	return user, nil
}

// writeDirect writes before the writer goroutine has started.
func (connection *connection) writeDirect(ctx context.Context, frame serverFrame) {
	_ = connection.write(ctx, encode(frame)) // a failure shows up on the next read
}
