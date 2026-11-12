package realtime

import (
	"context"
	"errors"
	"log/slog"
)

// readLoop handles frames after the handshake.
func (connection *connection) readLoop(ctx context.Context, session *gatewaySession, token string) *closeReason {
	limiter := newFrameLimiter()
	throttle := &typingThrottle{}
	interval := connection.gateway.options.HeartbeatInterval
	timedOut := &closeReason{closeTimedOut, "no heartbeat in time"}
	for {
		frame, reason := connection.read(ctx, interval*3/2, timedOut)
		if reason != nil {
			return reason
		}
		if !limiter.Allow() {
			return &closeReason{closeRateLimited, "too many frames"}
		}
		if frame.Type == typeTyping && connection.gateway.options.Typing != nil {
			if reason := connection.typing(ctx, session, frame, throttle); reason != nil {
				return reason
			}
			continue
		}
		if reason := connection.handle(ctx, session, token, frame); reason != nil {
			return reason
		}
	}
}

func (connection *connection) handle(ctx context.Context, session *gatewaySession, token string, frame clientFrame) *closeReason {
	switch frame.Type {
	case typeHeartbeat:
		// Re-checking the token here disconnects revoked sessions within
		// one interval, and keeps an active session from expiring.
		_, err := connection.gateway.authenticate(ctx, token)
		if errors.Is(err, errBadToken) {
			return &closeReason{closeBadToken, err.Error()}
		}
		if err != nil {
			connection.logger().ErrorContext(ctx, "gateway heartbeat check failed", slog.Any("error", err))
			return serverError()
		}
		connection.send(ctx, serverFrame{Type: typeHeartbeatAck, ID: frame.ID})
	case typeActivity:
		var data activityData
		if err := decodeData(frame, &data); err != nil {
			return &closeReason{closeMalformed, "malformed activity"}
		}
		connection.gateway.options.Hub.setIdle(session, data.Idle)
	case typeIdentify, typeResume:
		return &closeReason{closeAlreadyReady, "this connection is already identified"}
	default:
		connection.send(ctx, serverFrame{Type: typeError, ID: frame.ID, Data: errorData{Code: "unknown_type", Message: "unknown frame type " + frame.Type}})
	}
	return nil
}
