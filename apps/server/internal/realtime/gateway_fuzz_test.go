package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// expectedCloses are the ways a gateway may end a connection that sent
// it bad frames. An internal error, a hang or a crash is never one.
var expectedCloses = []websocket.StatusCode{ //nolint:gochecknoglobals // A fixed list the fuzz target checks against.
	closeMalformed, closeAlreadyReady, closeRateLimited, websocket.StatusMessageTooBig,
}

// FuzzGatewayFrames sends an identified connection a sequence of
// arbitrary frames (one per line of the input), then a heartbeat. The
// gateway must either still answer that heartbeat or have closed the
// connection with one of the expected codes.
func FuzzGatewayFrames(f *testing.F) {
	conversation := uuid.NewString()
	for _, seed := range []string{
		`{"type":"heartbeat","id":"1"}`,
		`{"type":"activity","data":{"idle":true}}` + "\n" + `{"type":"activity","data":{"idle":"yes"}}`,
		`{"type":"typing","data":{"conversationId":"` + conversation + `"}}`,
		`{"type":"typing","data":{"conversationId":"not an id"}}`,
		`{"type":"identify","data":{"token":"nxs_alice"}}`, `{"type":"resume","data":{}}`,
		`{"type":"nonsense","id":"9"}`, `{"type":"activity","data":[1]}`, "\xff", `[]`, `{}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		frames := bytes.Split(input, []byte("\n"))
		if len(frames) > 8 {
			frames = frames[:8]
		}
		fixture := withTyping(t, newGatewayFixture(t, time.Second), &fakeTyping{recipients: []uuid.UUID{uuid.New()}})
		device := fixture.dial(t)
		device.identify(aliceToken)
		for _, frame := range frames {
			if err := device.conn.Write(t.Context(), websocket.MessageText, frame); err != nil {
				break
			}
		}
		_ = device.conn.Write(t.Context(), websocket.MessageText, []byte(`{"type":"heartbeat","id":"probe"}`))
		alive, code := answeredProbe(t, device.conn)
		if !alive {
			require.True(t, slices.Contains(expectedCloses, code), "closed with %d", code)
		}
	})
}

// answeredProbe reads until the probe heartbeat is acknowledged, or the
// connection closes, and says which with the close code.
func answeredProbe(t *testing.T, conn *websocket.Conn) (bool, websocket.StatusCode) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for {
		_, message, err := conn.Read(ctx)
		if err != nil {
			code := websocket.CloseStatus(err)
			require.NotEqual(t, websocket.StatusCode(-1), code, "no close frame: %v", err)
			return false, code
		}
		var frame received
		require.NoError(t, json.Unmarshal(message, &frame))
		if frame.Type == typeHeartbeatAck && frame.ID == "probe" {
			return true, 0
		}
	}
}
