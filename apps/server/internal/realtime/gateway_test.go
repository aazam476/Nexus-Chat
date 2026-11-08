package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
)

var errDatabase = errors.New("database down")

// fakeAccounts knows tokens and users. failOn makes a step fail.
type fakeAccounts struct {
	mutex  sync.Mutex
	tokens map[string]uuid.UUID
	users  map[uuid.UUID]store.User
	failOn string
}

func (fake *fakeAccounts) Authenticate(_ context.Context, token string) (sessionstore.Session, error) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if fake.failOn == "Authenticate" {
		return sessionstore.Session{}, errDatabase
	}
	userID, found := fake.tokens[token]
	if !found {
		return sessionstore.Session{}, session.ErrInvalidSession
	}
	return sessionstore.Session{UserID: userID}, nil
}

func (fake *fakeAccounts) GetUserByID(_ context.Context, id uuid.UUID) (store.User, error) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if fake.failOn == "GetUserByID" {
		return store.User{}, errDatabase
	}
	return fake.users[id], nil
}

func (fake *fakeAccounts) revoke(token string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	delete(fake.tokens, token)
}

func (fake *fakeAccounts) fail(step string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.failOn = step
}

type gatewayFixture struct {
	server   *httptest.Server
	gateway  *Gateway
	hub      *Hub
	accounts *fakeAccounts
	user     store.User
}

const aliceToken = "nxs_alice"

func newGatewayFixture(t *testing.T, heartbeat time.Duration) gatewayFixture {
	t.Helper()
	user := store.User{ID: uuid.New(), Username: "alice", DisplayName: "Alice", Email: "alice@example.com", Status: "online"}
	accounts := &fakeAccounts{tokens: map[string]uuid.UUID{aliceToken: user.ID}, users: map[uuid.UUID]store.User{user.ID: user}}
	hub := NewHub(&testClock{now: testNow})
	gateway := NewGateway(GatewayOptions{
		Hub: hub, Sessions: accounts, Users: accounts, Logger: slog.New(slog.DiscardHandler),
		HeartbeatInterval: heartbeat, IdentifyTimeout: heartbeat,
	})
	server := httptest.NewServer(gateway)
	t.Cleanup(server.Close)
	return gatewayFixture{server: server, gateway: gateway, hub: hub, accounts: accounts, user: user}
}

type testClock struct {
	mutex sync.Mutex
	now   time.Time
}

func (clock *testClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.now
}

func (clock *testClock) advance(by time.Duration) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.now = clock.now.Add(by)
}

// client is a test device.
type client struct {
	t    *testing.T
	conn *websocket.Conn
}

type received struct {
	Type string          `json:"type"`
	Seq  uint64          `json:"seq"`
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}

func (fixture gatewayFixture) dial(t *testing.T) *client {
	t.Helper()
	address := "ws" + strings.TrimPrefix(fixture.server.URL, "http")
	conn, response, err := websocket.Dial(t.Context(), address, &websocket.DialOptions{Subprotocols: []string{Subprotocol}})
	require.NoError(t, err)
	if response.Body != nil {
		_ = response.Body.Close()
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	device := &client{t: t, conn: conn}
	assert.Equal(t, typeHello, device.receive().Type)
	return device
}

func (device *client) send(frame string) {
	device.t.Helper()
	require.NoError(device.t, device.conn.Write(device.t.Context(), websocket.MessageText, []byte(frame)))
}

func (device *client) receive() received {
	device.t.Helper()
	ctx, cancel := context.WithTimeout(device.t.Context(), 5*time.Second)
	defer cancel()
	_, message, err := device.conn.Read(ctx)
	require.NoError(device.t, err)
	var frame received
	require.NoError(device.t, json.Unmarshal(message, &frame))
	return frame
}

// receiveType skips frames until one of type wanted arrives.
func (device *client) receiveType(wanted string) received {
	device.t.Helper()
	for {
		if frame := device.receive(); frame.Type == wanted {
			return frame
		}
	}
}

// closedWith reads until the server closes the connection and returns the code.
func (device *client) closedWith() websocket.StatusCode {
	device.t.Helper()
	ctx, cancel := context.WithTimeout(device.t.Context(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := device.conn.Read(ctx); err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

func (device *client) identify(token string) received {
	device.t.Helper()
	device.send(`{"type":"identify","data":{"token":"` + token + `"}}`)
	return device.receiveType(typeReady)
}

func TestIdentifyReceivesReadyAndPresence(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	device := fixture.dial(t)

	ready := device.identify(aliceToken)

	assert.Equal(t, uint64(1), ready.Seq)
	var data struct {
		GatewaySessionID uuid.UUID `json:"gatewaySessionId"`
		User             struct{ Username string }
		Presence         Presence
	}
	require.NoError(t, json.Unmarshal(ready.Data, &data))
	assert.Equal(t, "alice", data.User.Username)
	assert.Equal(t, "online", data.Presence.Status)
	presence := device.receiveType(TypePresenceUpdate)
	assert.Equal(t, uint64(2), presence.Seq)
}

func TestHeartbeatsAreAcknowledged(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	device := fixture.dial(t)
	device.identify(aliceToken)

	device.send(`{"type":"heartbeat","id":"h1","data":{"lastSequence":2}}`)

	ack := device.receiveType(typeHeartbeatAck)
	assert.Equal(t, "h1", ack.ID)
}

func TestUnknownFramesGetAnError(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	device := fixture.dial(t)
	device.identify(aliceToken)

	device.send(`{"type":"dance","id":"x"}`)

	failure := device.receiveType(typeError)
	assert.Equal(t, "x", failure.ID)
	assert.Contains(t, string(failure.Data), "unknown_type")
}

func TestActivityMakesTheUserIdle(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	phone, laptop := fixture.dial(t), fixture.dial(t)
	phone.identify(aliceToken)
	laptop.identify(aliceToken)

	phone.send(`{"type":"activity","data":{"idle":true}}`)
	laptop.send(`{"type":"activity","data":{"idle":true}}`)

	for {
		update := phone.receiveType(TypePresenceUpdate)
		if strings.Contains(string(update.Data), `"idle"`) {
			break
		}
	}
	assert.Equal(t, "idle", fixture.hub.PresenceOf(fixture.user).Status)
	laptop.send(`{"type":"activity","data":{"idle":false}}`)
	require.Eventually(t, func() bool { return fixture.hub.PresenceOf(fixture.user).Status == "online" }, 5*time.Second, 10*time.Millisecond)
}

func TestUserChangesReachEveryDevice(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	device := fixture.dial(t)
	device.identify(aliceToken)
	changed := fixture.user
	changed.DisplayName, changed.Status = "Alice B", "dnd"

	fixture.hub.UserChanged(t.Context(), changed)

	update := device.receiveType(TypeUserUpdate)
	assert.Contains(t, string(update.Data), `"displayName":"Alice B"`)
	presence := device.receiveType(TypePresenceUpdate)
	assert.Contains(t, string(presence.Data), `"status":"dnd"`)
	fixture.hub.UserChanged(t.Context(), store.User{ID: uuid.New()}) // nobody connected: nothing happens
}

func TestResumeReplaysMissedEvents(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	first := fixture.dial(t)
	ready := first.identify(aliceToken)
	var data struct {
		GatewaySessionID string `json:"gatewaySessionId"`
	}
	require.NoError(t, json.Unmarshal(ready.Data, &data))
	last := first.receiveType(TypePresenceUpdate).Seq
	require.NoError(t, first.conn.Close(websocket.StatusNormalClosure, ""))
	require.Eventually(t, func() bool { return fixture.hub.PresenceOf(fixture.user).Status == "offline" }, 5*time.Second, 10*time.Millisecond)
	changed := fixture.user
	changed.Bio = "missed while away"
	fixture.hub.UserChanged(t.Context(), changed)

	second := fixture.dial(t)
	second.send(`{"type":"resume","data":{"token":"` + aliceToken + `","gatewaySessionId":"` + data.GatewaySessionID + `","lastSequence":` + jsonNumber(last) + `}}`)

	// The first missed event is the "offline" presence from the drop.
	offline := second.receive()
	assert.Equal(t, TypePresenceUpdate, offline.Type)
	assert.Equal(t, last+1, offline.Seq)
	replayed := second.receive()
	assert.Equal(t, TypeUserUpdate, replayed.Type)
	assert.Equal(t, last+2, replayed.Seq)
	assert.Contains(t, string(replayed.Data), "missed while away")
	assert.Equal(t, typeResumed, second.receiveType(typeResumed).Type)
}

func jsonNumber(value uint64) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestFailedResumeFallsBackToIdentify(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	device := fixture.dial(t)

	device.send(`{"type":"resume","data":{"token":"` + aliceToken + `","gatewaySessionId":"` + uuid.NewString() + `","lastSequence":0}}`)

	assert.Equal(t, typeInvalidSession, device.receive().Type)
	device.identify(aliceToken)
}

func TestResumingKicksTheOldConnection(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	first := fixture.dial(t)
	ready := first.identify(aliceToken)
	var data struct {
		GatewaySessionID string `json:"gatewaySessionId"`
	}
	require.NoError(t, json.Unmarshal(ready.Data, &data))

	second := fixture.dial(t)
	second.send(`{"type":"resume","data":{"token":"` + aliceToken + `","gatewaySessionId":"` + data.GatewaySessionID + `","lastSequence":1}}`)

	assert.Equal(t, closeReplaced, first.closedWith())
	second.receiveType(typeResumed)
}
