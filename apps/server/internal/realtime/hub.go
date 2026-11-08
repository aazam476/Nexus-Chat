package realtime

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// Hub knows every gateway session and delivers events to them. It is the
// in-memory EventBus and PresenceTracker of ADR 0012; all its state is
// guarded by one mutex, taken before any session's own mutex.
type Hub struct {
	clock    clock.Clock
	mutex    sync.Mutex
	sessions map[uuid.UUID]*gatewaySession
	users    map[uuid.UUID]*userState
}

// userState is what the hub knows about a user with at least one session.
type userState struct {
	user     store.User
	sessions map[uuid.UUID]*gatewaySession
	// shown is the presence last sent to the user's devices.
	shown Presence
}

// NewHub returns an empty Hub.
func NewHub(clock clock.Clock) *Hub {
	return &Hub{clock: clock, sessions: map[uuid.UUID]*gatewaySession{}, users: map[uuid.UUID]*userState{}}
}

type readyData struct {
	GatewaySessionID uuid.UUID       `json:"gatewaySessionId"`
	User             api.CurrentUser `json:"user"`
	Presence         Presence        `json:"presence"`
}

// link is how the hub reaches one connection: its queue, and a way to
// close it.
type link struct {
	outbox chan []byte
	kick   func()
}

// connect starts a new gateway session for user on a connection and sends
// it ready.
func (hub *Hub) connect(user store.User, connection link) *gatewaySession {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	session := newGatewaySession(user.ID)
	session.attach(0, connection.outbox, connection.kick)
	hub.sessions[session.id] = session
	state, found := hub.users[user.ID]
	if !found {
		state = &userState{user: user, sessions: map[uuid.UUID]*gatewaySession{}}
		hub.users[user.ID] = state
	}
	state.user = user
	state.sessions[session.id] = session
	presence := computePresence(hub.inputLocked(state), true)
	session.enqueue(hub.clock.Now(), func(seq uint64) []byte {
		data := readyData{GatewaySessionID: session.id, User: accounts.CurrentUserResponse(user), Presence: presence}
		return encode(serverFrame{Type: typeReady, Seq: seq, Data: data})
	})
	hub.publishPresenceLocked(state)
	return session
}

// resume reattaches session id, owned by userID, to a connection.
func (hub *Hub) resume(userID uuid.UUID, id uuid.UUID, lastSeq uint64, connection link) (*gatewaySession, int, bool) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	session, found := hub.sessions[id]
	if !found || session.userID != userID {
		return nil, 0, false
	}
	replayed, ok := session.attach(lastSeq, connection.outbox, connection.kick)
	if !ok {
		return nil, 0, false
	}
	hub.publishPresenceLocked(hub.users[userID])
	return session, replayed, true
}

// disconnect detaches a connection. The session stays resumable.
func (hub *Hub) disconnect(session *gatewaySession, outbox chan []byte) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	session.detach(outbox, hub.clock.Now())
	// Presence is recomputed even if a newer connection took over, since
	// a session dropped for being slow is already detached.
	if state, found := hub.users[session.userID]; found {
		hub.publishPresenceLocked(state)
	}
}

// setIdle records whether the device on session is inactive.
func (hub *Hub) setIdle(session *gatewaySession, idle bool) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	session.mutex.Lock()
	session.idle = idle
	session.mutex.Unlock()
	if state, found := hub.users[session.userID]; found {
		hub.publishPresenceLocked(state)
	}
}

// UserChanged tells the user's devices about a change to their account
// and updates their presence. It implements profiles.ChangeListener.
func (hub *Hub) UserChanged(_ context.Context, user store.User) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	state, found := hub.users[user.ID]
	if !found {
		return // no device is connected
	}
	state.user = user
	update := map[string]api.CurrentUser{"user": accounts.CurrentUserResponse(user)}
	hub.sendLocked(state, TypeUserUpdate, update)
	hub.publishPresenceLocked(state)
}

// PresenceOf is the presence other users see for user right now.
func (hub *Hub) PresenceOf(user store.User) Presence {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	input := presenceInput{user: user, now: hub.clock.Now()}
	if state, found := hub.users[user.ID]; found {
		input = hub.inputLocked(state)
		input.user = user
	}
	return computePresence(input, false)
}

// sweep forgets sessions past their resume window.
func (hub *Hub) sweep() {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	now := hub.clock.Now()
	for id, session := range hub.sessions {
		if !session.expired(now) {
			continue
		}
		delete(hub.sessions, id)
		state := hub.users[session.userID]
		delete(state.sessions, id)
		if len(state.sessions) == 0 {
			delete(hub.users, session.userID)
		}
	}
}

// inputLocked gathers what the user's presence depends on.
func (hub *Hub) inputLocked(state *userState) presenceInput {
	input := presenceInput{user: state.user, allIdle: true, now: hub.clock.Now()}
	for _, session := range state.sessions {
		session.mutex.Lock()
		if session.outbox != nil {
			input.connected++
			input.allIdle = input.allIdle && session.idle
		}
		session.mutex.Unlock()
	}
	return input
}

// publishPresenceLocked sends a presence_update when what the user's
// devices should show has changed.
func (hub *Hub) publishPresenceLocked(state *userState) {
	presence := computePresence(hub.inputLocked(state), true)
	if samePresence(presence, state.shown) {
		return
	}
	state.shown = presence
	hub.sendLocked(state, TypePresenceUpdate, presence)
}

// sendLocked delivers an event to every session of the user.
func (hub *Hub) sendLocked(state *userState, eventType string, data any) {
	now := hub.clock.Now()
	for _, session := range state.sessions {
		session.enqueue(now, func(seq uint64) []byte {
			return encode(serverFrame{Type: eventType, Seq: seq, Data: data})
		})
	}
}
