package realtime

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Resume limits (docs/realtime-protocol.md §3).
const (
	replayBufferSize = 500
	resumeWindow     = 2 * time.Minute
	// outboxSize holds a full replay plus room for new events, so replaying
	// never blocks.
	outboxSize = replayBufferSize + 100
)

// event is a sequenced frame, kept for replay.
type event struct {
	seq     uint64
	encoded []byte
}

// gatewaySession is one device's stream of events. It outlives a dropped
// connection for resumeWindow so the device can pick up where it was.
type gatewaySession struct {
	id     uuid.UUID
	userID uuid.UUID

	mutex sync.Mutex
	// nextSeq is the sequence number of the next event.
	nextSeq uint64
	// buffer holds the most recent events, oldest first.
	buffer []event
	// outbox is the attached connection's queue, nil while detached.
	outbox chan []byte
	// kick closes the attached connection; called when it falls behind or
	// a newer connection resumes the session.
	kick           func()
	idle           bool
	disconnectedAt time.Time
}

func newGatewaySession(userID uuid.UUID) *gatewaySession {
	return &gatewaySession{id: uuid.Must(uuid.NewV7()), userID: userID, nextSeq: 1}
}

// enqueue numbers an event, keeps it for replay and hands it to the
// connection, if one is attached. build receives the event's sequence
// number and returns its encoded frame.
func (session *gatewaySession) enqueue(now time.Time, build func(seq uint64) []byte) {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	next := event{seq: session.nextSeq, encoded: build(session.nextSeq)}
	session.nextSeq++
	session.buffer = append(session.buffer, next)
	if len(session.buffer) > replayBufferSize {
		session.buffer = session.buffer[len(session.buffer)-replayBufferSize:]
	}
	if session.outbox == nil {
		return
	}
	select {
	case session.outbox <- next.encoded:
	default:
		// The device is not reading. Rather than block every publisher,
		// drop the connection; the device resumes and gets the buffer.
		session.detachLocked()
		session.disconnectedAt = now
	}
}

// attach connects a new connection. Events after lastSeq are replayed
// first; ok is false when some of them are no longer buffered, or lastSeq
// is from the future. An earlier connection is kicked.
func (session *gatewaySession) attach(lastSeq uint64, outbox chan []byte, kick func()) (int, bool) {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	oldest := session.nextSeq
	if len(session.buffer) > 0 {
		oldest = session.buffer[0].seq
	}
	if lastSeq >= session.nextSeq || lastSeq+1 < oldest {
		return 0, false
	}
	session.detachLocked()
	replayed := 0
	for _, buffered := range session.buffer {
		if buffered.seq > lastSeq {
			outbox <- buffered.encoded // fits: outboxSize > replayBufferSize
			replayed++
		}
	}
	session.outbox, session.kick, session.idle = outbox, kick, false
	return replayed, true
}

// detach disconnects the connection whose outbox this is. A connection
// replaced by a resume no longer owns the session and changes nothing.
func (session *gatewaySession) detach(outbox chan []byte, now time.Time) bool {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	if session.outbox != outbox || outbox == nil {
		return false
	}
	session.outbox, session.kick = nil, nil
	session.disconnectedAt = now
	return true
}

// detachLocked kicks the attached connection, if any.
func (session *gatewaySession) detachLocked() {
	if session.kick != nil {
		session.kick()
	}
	session.outbox, session.kick = nil, nil
}

// expired reports whether a detached session is past its resume window.
func (session *gatewaySession) expired(now time.Time) bool {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	return session.outbox == nil && now.Sub(session.disconnectedAt) > resumeWindow
}
