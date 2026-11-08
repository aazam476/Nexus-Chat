package realtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Limits on client frames (docs/realtime-protocol.md §2).
const (
	maximumFrameBytes = 16 << 10
	maximumTypeLength = 64
)

// Client frame types.
const (
	typeIdentify  = "identify"
	typeResume    = "resume"
	typeHeartbeat = "heartbeat"
	typeActivity  = "activity"
)

// Server frame types.
const (
	typeHello          = "hello"
	typeReady          = "ready"
	typeResumed        = "resumed"
	typeInvalidSession = "invalid_session"
	typeHeartbeatAck   = "heartbeat_ack"
	typeError          = "error"
	// TypePresenceUpdate and TypeUserUpdate are events.
	TypePresenceUpdate = "presence_update"
	TypeUserUpdate     = "user_update"
)

var errMalformed = errors.New("malformed frame")

// clientFrame is a frame from a device. Data stays raw until the type is
// known.
type clientFrame struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"`
	Data json.RawMessage `json:"data"`
}

// decodeClientFrame parses one frame from the network. Anything that is not
// a JSON object with a short string type is rejected.
func decodeClientFrame(message []byte) (clientFrame, error) {
	if len(message) > maximumFrameBytes || !utf8.Valid(message) {
		return clientFrame{}, errMalformed
	}
	var frame clientFrame
	decoder := json.NewDecoder(bytes.NewReader(message))
	if err := decoder.Decode(&frame); err != nil || decoder.More() {
		return clientFrame{}, errMalformed
	}
	if frame.Type == "" || len(frame.Type) > maximumTypeLength || len(frame.ID) > maximumTypeLength {
		return clientFrame{}, errMalformed
	}
	trimmed := bytes.TrimSpace(frame.Data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		frame.Data = json.RawMessage("{}")
	}
	return frame, nil
}

// decodeData reads a frame's data into target, which must be an object.
func decodeData(frame clientFrame, target any) error {
	if bytes.TrimSpace(frame.Data)[0] != '{' {
		return fmt.Errorf("%w: data must be an object", errMalformed)
	}
	if err := json.Unmarshal(frame.Data, target); err != nil {
		return fmt.Errorf("%w: %s", errMalformed, frame.Type)
	}
	return nil
}

type identifyData struct {
	Token string `json:"token"`
}

type resumeData struct {
	Token            string    `json:"token"`
	GatewaySessionID uuid.UUID `json:"gatewaySessionId"`
	LastSequence     uint64    `json:"lastSequence"`
}

type activityData struct {
	Idle bool `json:"idle"`
}

// serverFrame is a frame to a device. Seq is set only on events.
type serverFrame struct {
	Type string `json:"type"`
	Seq  uint64 `json:"seq,omitempty"`
	ID   string `json:"id,omitempty"`
	Data any    `json:"data"`
}

// encode turns a frame into its JSON text. Frame data is always made of
// structs, strings, numbers and API types, which always encode.
func encode(frame serverFrame) []byte {
	if frame.Data == nil {
		frame.Data = struct{}{}
	}
	encoded, _ := json.Marshal(frame)
	return encoded
}

type helloData struct {
	HeartbeatIntervalMs int64 `json:"heartbeatIntervalMs"`
	ProtocolVersion     int   `json:"protocolVersion"`
}

type errorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type resumedData struct {
	Replayed int `json:"replayed"`
}
