package ratelimit

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// manualClock is a clock the test moves forward by hand.
type manualClock struct {
	mutex sync.Mutex
	now   time.Time
}

func (manual *manualClock) Now() time.Time {
	manual.mutex.Lock()
	defer manual.mutex.Unlock()
	return manual.now
}

func (manual *manualClock) advance(duration time.Duration) {
	manual.mutex.Lock()
	defer manual.mutex.Unlock()
	manual.now = manual.now.Add(duration)
}

func newClock() *manualClock {
	return &manualClock{now: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)}
}

func TestAllowsBurstThenRefills(t *testing.T) {
	clock := newClock()
	limiter := New(PerMinute(6, 3), clock) // one token every 10 s

	assert.True(t, limiter.Allow("a"))
	assert.True(t, limiter.Allow("a"))
	assert.True(t, limiter.Allow("a"))
	assert.False(t, limiter.Allow("a"), "burst of 3 is used up")

	clock.advance(10 * time.Second)
	assert.True(t, limiter.Allow("a"), "one token refilled")
	assert.False(t, limiter.Allow("a"))
}

func TestKeysAreIndependent(t *testing.T) {
	limiter := New(PerMinute(1, 1), newClock())

	assert.True(t, limiter.Allow("a"))
	assert.False(t, limiter.Allow("a"))
	assert.True(t, limiter.Allow("b"))
}

func TestIdleKeysAreForgotten(t *testing.T) {
	clock := newClock()
	limiter := New(PerMinute(1, 1), clock)
	limiter.Allow("old")
	clock.advance(idleTimeout / 2)
	limiter.Allow("recent")

	clock.advance(idleTimeout / 2)
	limiter.Allow("trigger")

	// "old" was idle for a full timeout and is gone; "recent" and
	// "trigger" remain.
	assert.Equal(t, 2, limiter.trackedKeys())
}

func TestSweepRunsAtMostOncePerTimeout(t *testing.T) {
	clock := newClock()
	limiter := New(PerMinute(1, 1), clock)
	for index := range 100 {
		limiter.Allow(strconv.Itoa(index))
	}

	clock.advance(idleTimeout - time.Second)
	limiter.Allow("x")

	assert.Equal(t, 101, limiter.trackedKeys(), "no sweep before the timeout")
}

func TestConcurrentUse(t *testing.T) {
	limiter := New(PerMinute(60, 50), newClock())
	var waitGroup sync.WaitGroup
	allowed := make(chan bool, 100)
	for range 100 {
		waitGroup.Go(func() { allowed <- limiter.Allow("shared") })
	}
	waitGroup.Wait()
	close(allowed)

	count := 0
	for ok := range allowed {
		if ok {
			count++
		}
	}
	assert.Equal(t, 50, count, "exactly the burst is allowed")
}
