package realtime

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
)

type fakeStatuses struct {
	cleared  []uuid.UUID
	users    map[uuid.UUID]store.User
	clearErr error
	userErr  error
}

func (fake *fakeStatuses) ClearExpiredCustomStatuses(context.Context, time.Time) ([]uuid.UUID, error) {
	return fake.cleared, fake.clearErr
}

func (fake *fakeStatuses) GetUserByID(_ context.Context, id uuid.UUID) (store.User, error) {
	return fake.users[id], fake.userErr
}

func TestSweepForgetsSessionsAfterTheResumeWindow(t *testing.T) {
	clock := &testClock{now: testNow}
	hub := NewHub(clock)
	user := store.User{ID: uuid.New(), Status: "online"}
	outbox := make(chan []byte, outboxSize)
	session := hub.connect(user, link{outbox: outbox, kick: func() {}})
	hub.disconnect(session, outbox)
	sweeper := NewSweeper(hub, &fakeStatuses{}, slog.New(slog.DiscardHandler))

	sweeper.Sweep(t.Context())
	_, _, ok := hub.resume(user.ID, session.id, 1, link{outbox: make(chan []byte, outboxSize), kick: func() {}})
	require.True(t, ok, "still within the window")
	hub.disconnect(session, nil) // a connection that was never attached changes nothing

	clock.advance(resumeWindow + time.Minute)
	_, _, ok = hub.resume(uuid.New(), session.id, 1, link{})
	assert.False(t, ok, "another user cannot resume it")
	hub.disconnect(session, session.outbox)
	clock.advance(resumeWindow + time.Second)
	sweeper.Sweep(t.Context())
	_, _, ok = hub.resume(user.ID, session.id, 1, link{outbox: make(chan []byte, outboxSize), kick: func() {}})
	assert.False(t, ok, "forgotten")
	assert.Equal(t, "offline", hub.PresenceOf(user).Status)
	hub.disconnect(session, nil) // the user is gone from the hub: nothing to update
}

func TestSweepClearsExpiredCustomStatuses(t *testing.T) {
	hub := NewHub(&testClock{now: testNow})
	user := store.User{ID: uuid.New(), Status: "online"}
	outbox := make(chan []byte, outboxSize)
	hub.connect(user, link{outbox: outbox, kick: func() {}})
	drain(outbox)
	sweeper := NewSweeper(hub, &fakeStatuses{cleared: []uuid.UUID{user.ID}, users: map[uuid.UUID]store.User{user.ID: user}}, slog.New(slog.DiscardHandler))

	sweeper.Sweep(t.Context())

	frames := drain(outbox)
	require.NotEmpty(t, frames)
	assert.Contains(t, frames[0], TypeUserUpdate)
}

func TestSweepLogsFailures(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	hub := NewHub(&testClock{now: testNow})

	NewSweeper(hub, &fakeStatuses{clearErr: errDatabase}, logger).Sweep(t.Context())
	NewSweeper(hub, &fakeStatuses{cleared: []uuid.UUID{uuid.New()}, userErr: errDatabase}, logger).Sweep(t.Context())

	assert.Contains(t, logs.String(), "could not clear expired custom statuses")
	assert.Contains(t, logs.String(), "could not reload user")
}

func TestRunSweepsUntilCancelled(t *testing.T) {
	statuses := &countingStatuses{}
	sweeper := NewSweeper(NewHub(&testClock{now: testNow}), statuses, slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() { // ends when Run returns, after cancel below
		sweeper.Run(ctx, time.Millisecond)
		close(done)
	}()
	require.Eventually(t, func() bool { return statuses.calls() > 1 }, 5*time.Second, time.Millisecond)
	cancel()
	<-done
	assert.Equal(t, 30*time.Second, SweepInterval())
}

type countingStatuses struct {
	fakeStatuses
	mutex sync.Mutex
	count int
}

func (fake *countingStatuses) ClearExpiredCustomStatuses(context.Context, time.Time) ([]uuid.UUID, error) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.count++
	return nil, nil
}

func (fake *countingStatuses) calls() int {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return fake.count
}
