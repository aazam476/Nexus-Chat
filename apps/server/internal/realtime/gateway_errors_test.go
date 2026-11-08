package realtime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionsAreClosedForProtocolErrors(t *testing.T) {
	tests := []struct {
		name   string
		frames []string
		setup  func(gatewayFixture)
		want   websocket.StatusCode
	}{
		{"bad token", []string{`{"type":"identify","data":{"token":"nxs_wrong"}}`}, nil, closeBadToken},
		{"not JSON", []string{`hello`}, nil, closeMalformed},
		{"identify data not an object", []string{`{"type":"identify","data":[1]}`}, nil, closeMalformed},
		{"resume data not an object", []string{`{"type":"resume","data":[1]}`}, nil, closeMalformed},
		{"resume with bad token", []string{`{"type":"resume","data":{"token":"nxs_wrong"}}`}, nil, closeBadToken},
		{"heartbeat first", []string{`{"type":"heartbeat"}`}, nil, closeNotIdentified},
		{"resume three times", []string{
			`{"type":"resume","data":{"token":"` + aliceToken + `"}}`, `{"type":"resume","data":{"token":"` + aliceToken + `"}}`,
			`{"type":"resume","data":{"token":"` + aliceToken + `"}}`,
		}, nil, closeNotIdentified},
		{"database down", []string{`{"type":"identify","data":{"token":"` + aliceToken + `"}}`},
			func(f gatewayFixture) { f.accounts.fail("Authenticate") }, websocket.StatusInternalError},
		{"user unreadable", []string{`{"type":"identify","data":{"token":"` + aliceToken + `"}}`},
			func(f gatewayFixture) { f.accounts.fail("GetUserByID") }, websocket.StatusInternalError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGatewayFixture(t, time.Second)
			if test.setup != nil {
				test.setup(fixture)
			}
			device := fixture.dial(t)
			for _, frame := range test.frames {
				device.send(frame)
			}

			assert.Equal(t, test.want, device.closedWith())
		})
	}
}

func TestIdentifiedConnectionsAreClosedForMisbehavior(t *testing.T) {
	tests := []struct {
		name string
		act  func(gatewayFixture, *client)
		want websocket.StatusCode
	}{
		{"identify twice", func(_ gatewayFixture, device *client) { device.send(`{"type":"identify","data":{}}`) }, closeAlreadyReady},
		{"binary frame", func(_ gatewayFixture, device *client) {
			require.NoError(t, device.conn.Write(t.Context(), websocket.MessageBinary, []byte("{}")))
		}, closeMalformed},
		{"bad activity", func(_ gatewayFixture, device *client) { device.send(`{"type":"activity","data":[true]}`) }, closeMalformed},
		{"flood", func(_ gatewayFixture, device *client) {
			for range framesPerWindow + 1 {
				device.send(`{"type":"nothing"}`)
			}
		}, closeRateLimited},
		{"revoked", func(f gatewayFixture, device *client) {
			f.accounts.revoke(aliceToken)
			device.send(`{"type":"heartbeat"}`)
		}, closeBadToken},
		{"database down on heartbeat", func(f gatewayFixture, device *client) {
			f.accounts.fail("Authenticate")
			device.send(`{"type":"heartbeat"}`)
		}, websocket.StatusInternalError},
		{"silent", func(gatewayFixture, *client) {}, closeTimedOut},
		{"server closes", func(f gatewayFixture, _ *client) { f.gateway.Close() }, websocket.StatusGoingAway},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGatewayFixture(t, 300*time.Millisecond)
			device := fixture.dial(t)
			device.identify(aliceToken)

			test.act(fixture, device)

			assert.Equal(t, test.want, device.closedWith())
		})
	}
}

func TestSilentConnectionsAreClosedBeforeIdentify(t *testing.T) {
	fixture := newGatewayFixture(t, 200*time.Millisecond)
	device := fixture.dial(t)

	assert.Equal(t, closeNotIdentified, device.closedWith())
}

func TestTheSubprotocolIsRequired(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	address := "ws" + strings.TrimPrefix(fixture.server.URL, "http")
	conn, response, err := websocket.Dial(t.Context(), address, nil)
	require.NoError(t, err)
	if response.Body != nil {
		_ = response.Body.Close()
	}
	_, _, err = conn.Read(t.Context())

	assert.Equal(t, websocket.StatusPolicyViolation, websocket.CloseStatus(err))
}

func TestAClosedGatewayRefusesConnections(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	fixture.gateway.Close()

	response, err := http.Get(fixture.server.URL) //nolint:noctx // a test request
	require.NoError(t, err)
	_ = response.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, response.StatusCode)

	plain, err := http.Get(fixture.server.URL) //nolint:noctx // a test request
	require.NoError(t, err)
	_ = plain.Body.Close()
}

func TestPlainRequestsAreNotUpgraded(t *testing.T) {
	gateway := NewGateway(GatewayOptions{Hub: NewHub(&testClock{})})
	recorder := httptest.NewRecorder()

	gateway.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/gateway", nil))

	assert.Equal(t, http.StatusUpgradeRequired, recorder.Code)
}
