package realtime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzDecodeClientFrame checks that no input crashes the parser, and that
// every frame it accepts has a usable type and object-shaped data.
func FuzzDecodeClientFrame(f *testing.F) {
	for _, seed := range []string{
		`{"type":"identify","data":{"token":"nxs_x"}}`,
		`{"type":"heartbeat","id":"1","data":{"lastSequence":3}}`,
		`{"type":"activity"}`, `{"type":"x","data":null}`, `{"type":"x","data":[1]}`,
		`[]`, `null`, `{"type":1}`, `{"type":"a"}{"type":"b"}`, "\xff", ``,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, message []byte) {
		frame, err := decodeClientFrame(message)
		if err != nil {
			return
		}
		assert.NotEmpty(t, frame.Type)
		assert.LessOrEqual(t, len(frame.Type), maximumTypeLength)
		require.True(t, json.Valid(frame.Data))
		var activity activityData
		_ = decodeData(frame, &activity) // must not panic
	})
}

func TestDecodeClientFrame(t *testing.T) {
	frame, err := decodeClientFrame([]byte(`{"type":"heartbeat","id":"7","data":{"lastSequence":3},"extra":true}`))
	require.NoError(t, err)
	assert.Equal(t, "heartbeat", frame.Type)
	assert.Equal(t, "7", frame.ID)

	frame, err = decodeClientFrame([]byte(`{"type":"activity"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(frame.Data))

	long := make([]byte, maximumFrameBytes+1)
	for _, message := range [][]byte{
		long, []byte("\xff"), []byte(`[]`), []byte(`{"type":""}`), []byte(`{"type":"a"} {}`),
		[]byte(`{"type":"` + string(make([]byte, 65)) + `"}`), []byte(`{"type":"a","id":"` + string(make([]byte, 65)) + `"}`),
	} {
		_, err := decodeClientFrame(message)
		assert.ErrorIs(t, err, errMalformed, string(message))
	}
}

func TestDecodeData(t *testing.T) {
	var identify identifyData
	require.NoError(t, decodeData(clientFrame{Type: "identify", Data: json.RawMessage(`{"token":"nxs_a"}`)}, &identify))
	assert.Equal(t, "nxs_a", identify.Token)

	require.ErrorIs(t, decodeData(clientFrame{Data: json.RawMessage(`[1]`)}, &identify), errMalformed)
	assert.ErrorIs(t, decodeData(clientFrame{Data: json.RawMessage(`{"token":1}`)}, &identify), errMalformed)
}

func TestEncodeFrames(t *testing.T) {
	assert.JSONEq(t, `{"type":"heartbeat_ack","data":{}}`, string(encode(serverFrame{Type: typeHeartbeatAck})))
	assert.JSONEq(t, `{"type":"presence_update","seq":4,"data":{"a":1}}`, string(encode(serverFrame{Type: TypePresenceUpdate, Seq: 4, Data: map[string]int{"a": 1}})))
}
