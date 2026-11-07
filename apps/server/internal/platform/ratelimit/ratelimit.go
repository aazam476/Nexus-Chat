// Package ratelimit limits how often each key (an IP address, account or
// device) may do something, using a token bucket per key.
//
// A token bucket holds up to Burst tokens and refills at Rate tokens per
// second; each action spends one. Start reading at [New] and
// [Limiter.Allow].
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// idleTimeout is how long a key may go unused before its bucket is
// forgotten. By then it has refilled completely, so forgetting it changes
// nothing except freeing memory.
const idleTimeout = 30 * time.Minute

// Policy describes one limit.
type Policy struct {
	// Rate is the refill speed in tokens per second.
	Rate rate.Limit
	// Burst is the bucket size: how many actions may happen at once.
	Burst int
}

// PerMinute returns a Policy allowing count actions per minute on average,
// with bursts of up to burst.
func PerMinute(count int, burst int) Policy {
	return Policy{Rate: rate.Limit(float64(count) / 60), Burst: burst}
}

type bucket struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

// Limiter applies one Policy separately to every key. It is safe for
// concurrent use.
type Limiter struct {
	policy    Policy
	clock     clock.Clock
	mutex     sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

// New returns a Limiter enforcing policy, reading time from clock.
func New(policy Policy, clock clock.Clock) *Limiter {
	return &Limiter{policy: policy, clock: clock, buckets: map[string]*bucket{}, lastSweep: clock.Now()}
}

// Allow spends one token for key and reports whether one was available.
func (limiter *Limiter) Allow(key string) bool {
	now := limiter.clock.Now()
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	limiter.sweep(now)
	entry, exists := limiter.buckets[key]
	if !exists {
		entry = &bucket{limiter: rate.NewLimiter(limiter.policy.Rate, limiter.policy.Burst)}
		limiter.buckets[key] = entry
	}
	entry.lastUsed = now
	return entry.limiter.AllowN(now, 1)
}

// sweep forgets idle buckets. It runs inside Allow at most once per
// idleTimeout, so no background goroutine is needed. The caller holds the
// mutex.
func (limiter *Limiter) sweep(now time.Time) {
	if now.Sub(limiter.lastSweep) < idleTimeout {
		return
	}
	for key, entry := range limiter.buckets {
		if now.Sub(entry.lastUsed) >= idleTimeout {
			delete(limiter.buckets, key)
		}
	}
	limiter.lastSweep = now
}

// trackedKeys returns how many keys currently have a bucket (for tests).
func (limiter *Limiter) trackedKeys() int {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	return len(limiter.buckets)
}
