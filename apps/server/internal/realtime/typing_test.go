package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTyping answers TypingRecipients with fixed recipients, or fails.
type fakeTyping struct {
	mutex      sync.Mutex
	recipients []uuid.UUID
	fail       bool
	calls      []uuid.UUID
}

func (fake *fakeTyping) TypingRecipients(_ context.Context, conversationID uuid.UUID, _ uuid.UUID) ([]uuid.UUID, error) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.calls = append(fake.calls, conversationID)
	if fake.fail {
		return nil, errDatabase
	}
	return fake.recipients, nil
}

func (fake *fakeTyping) callCount() int {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return len(fake.calls)
}

// withTyping is the fixture's hub and accounts behind a gateway that relays typing.
func withTyping(t *testing.T, fixture gatewayFixture, relay TypingRelay) gatewayFixture {
	t.Helper()
	gateway := NewGateway(GatewayOptions{
		Hub: fixture.hub, Sessions: fixture.accounts, Users: fixture.accounts, Logger: slog.New(slog.DiscardHandler),
		Typing: relay, HeartbeatInterval: time.Second, IdentifyTimeout: time.Second,
	})
	server := httptest.NewServer(gateway)
	t.Cleanup(server.Close)
	fixture.server, fixture.gateway = server, gateway
	return fixture
}

func TestTypingReachesTheOtherMembersOncePerFewSeconds(t *testing.T) {
	base := newGatewayFixture(t, time.Second)
	relay := &fakeTyping{recipients: []uuid.UUID{base.user.ID}}
	fixture := withTyping(t, base, relay)
	phone, laptop := fixture.dial(t), fixture.dial(t)
	phone.identify(aliceToken)
	laptop.identify(aliceToken)
	conversation := uuid.New()
	typing := `{"type":"typing","data":{"conversationId":"` + conversation.String() + `"}}`

	phone.send(typing)
	frame := laptop.receiveType(TypeTypingStart)
	var data typingStartData
	require.NoError(t, json.Unmarshal(frame.Data, &data))
	assert.Equal(t, typingStartData{ConversationID: conversation, UserID: fixture.user.ID}, data)

	phone.send(typing) // too soon: not relayed
	phone.send(`{"type":"heartbeat","id":"h","data":{"lastSequence":0}}`)
	phone.receiveType(typeHeartbeatAck)
	assert.Equal(t, 1, relay.callCount())
	fixture.hub.clock.(*testClock).advance(typingRepeat)
	phone.send(typing)
	laptop.receiveType(TypeTypingStart)
	assert.Equal(t, 2, relay.callCount())
}

func TestTypingFramesAreChecked(t *testing.T) {
	base := newGatewayFixture(t, time.Second)
	relay := &fakeTyping{fail: true}
	fixture := withTyping(t, base, relay)
	device := fixture.dial(t)
	device.identify(aliceToken)

	device.send(`{"type":"typing","id":"t1","data":{"conversationId":"not-a-uuid"}}`)
	failure := device.receiveType(typeError)
	assert.Equal(t, "t1", failure.ID)
	assert.Contains(t, string(failure.Data), "invalid_conversation")

	// A failing lookup is logged; the connection carries on.
	device.send(`{"type":"typing","data":{"conversationId":"` + uuid.NewString() + `"}}`)
	device.send(`{"type":"heartbeat","id":"h","data":{"lastSequence":0}}`)
	device.receiveType(typeHeartbeatAck)
	assert.Equal(t, 1, relay.callCount())

	device.send(`{"type":"typing","data":[1]}`)
	assert.Equal(t, closeMalformed, device.closedWith())
}

func TestTypingIsUnknownWithoutARelay(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	device := fixture.dial(t)
	device.identify(aliceToken)

	device.send(`{"type":"typing","data":{"conversationId":"` + uuid.NewString() + `"}}`)

	assert.Contains(t, string(device.receiveType(typeError).Data), "unknown_type")
}

func TestTheTypingThrottleForgetsOldConversations(t *testing.T) {
	throttle := &typingThrottle{}
	first := uuid.New()
	require.True(t, throttle.allow(first, testNow))
	require.False(t, throttle.allow(first, testNow.Add(typingRepeat-time.Millisecond)))
	for range typingRemembered {
		throttle.allow(uuid.New(), testNow)
	}
	assert.True(t, throttle.allow(first, testNow), "forgotten once too many were remembered")
}
