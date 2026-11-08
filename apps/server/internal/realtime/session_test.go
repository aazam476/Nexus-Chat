package realtime

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func frameFor(seq uint64) []byte { return []byte(fmt.Sprint(seq)) }

func drain(outbox chan []byte) []string {
	var frames []string
	for {
		select {
		case frame := <-outbox:
			frames = append(frames, string(frame))
		default:
			return frames
		}
	}
}

func TestEventsAreNumberedAndReplayed(t *testing.T) {
	session := newGatewaySession(uuid.New())
	first := make(chan []byte, outboxSize)
	_, ok := session.attach(0, first, func() {})
	require.True(t, ok)
	for range 3 {
		session.enqueue(testNow, frameFor)
	}
	assert.Equal(t, []string{"1", "2", "3"}, drain(first))

	require.True(t, session.detach(first, testNow))
	assert.False(t, session.detach(first, testNow), "already detached")
	session.enqueue(testNow, frameFor) // arrives while disconnected

	second := make(chan []byte, outboxSize)
	replayed, ok := session.attach(2, second, func() {})
	require.True(t, ok)
	assert.Equal(t, 2, replayed)
	assert.Equal(t, []string{"3", "4"}, drain(second))
}

func TestResumeFailsWhenEventsAreGoneOrFromTheFuture(t *testing.T) {
	session := newGatewaySession(uuid.New())
	for range replayBufferSize + 10 {
		session.enqueue(testNow, frameFor)
	}
	_, ok := session.attach(5, make(chan []byte, outboxSize), func() {})
	assert.False(t, ok, "event 6 is no longer buffered")
	_, ok = session.attach(replayBufferSize+10+1, make(chan []byte, outboxSize), func() {})
	assert.False(t, ok, "from the future")
	replayed, ok := session.attach(10, make(chan []byte, outboxSize), func() {})
	require.True(t, ok, "event 11 is the oldest kept")
	assert.Equal(t, replayBufferSize, replayed)
}

func TestAResumeKicksTheEarlierConnection(t *testing.T) {
	session := newGatewaySession(uuid.New())
	kicked := false
	first := make(chan []byte, outboxSize)
	session.attach(0, first, func() { kicked = true })

	_, ok := session.attach(0, make(chan []byte, outboxSize), func() {})

	require.True(t, ok)
	assert.True(t, kicked)
	assert.False(t, session.detach(first, testNow), "the old connection no longer owns the session")
}

func TestASlowConnectionIsDropped(t *testing.T) {
	session := newGatewaySession(uuid.New())
	kicked := false
	full := make(chan []byte, 1)
	session.attach(0, full, func() { kicked = true })

	session.enqueue(testNow, frameFor)
	session.enqueue(testNow, frameFor)

	assert.True(t, kicked)
	assert.False(t, session.expired(testNow.Add(resumeWindow)))
	assert.True(t, session.expired(testNow.Add(resumeWindow+time.Second)))
	replayed, ok := session.attach(1, make(chan []byte, outboxSize), func() {})
	require.True(t, ok, "the device resumes and gets what it missed")
	assert.Equal(t, 1, replayed)
}

func TestAnAttachedSessionNeverExpires(t *testing.T) {
	session := newGatewaySession(uuid.New())
	session.attach(0, make(chan []byte, 1), func() {})

	assert.False(t, session.expired(testNow.Add(time.Hour)))
}
