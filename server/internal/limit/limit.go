// Package limit is an in-memory token-bucket rate limiter keyed by string
// (client IP). Idle keys are evicted so memory cannot grow without bound.
package limit

import (
	"context"
	"math"
	"sync"
	"time"
)

// MaxKeys bounds the map. When it is full and a sweep frees nothing, new keys
// are refused until buckets refill (the limiter fails closed).
const MaxKeys = 100_000

const epsilon = 1e-9

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter hands out tokens at rate per second up to burst per key.
type Limiter struct {
	// Now is the clock; set it before first use (tests inject a fake one).
	Now func() time.Time

	rate  float64
	burst float64
	max   int
	mu    sync.Mutex
	m     map[string]*bucket
}

// New returns a limiter that refills rate tokens per second per key, holding at
// most burst tokens. Panics on a non-positive rate or burst (programmer error).
func New(rate float64, burst int) *Limiter {
	if rate <= 0 || burst < 1 {
		panic("limit: rate and burst must be positive")
	}
	return &Limiter{Now: time.Now, rate: rate, burst: float64(burst), max: MaxKeys, m: map[string]*bucket{}}
}

// Allow takes one token for key. When none is left it returns false and how
// long to wait until one is available.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	now := l.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.m[key]
	if b == nil {
		if len(l.m) >= l.max {
			l.sweepLocked(now)
			if len(l.m) >= l.max {
				return false, l.wait(1)
			}
		}
		b = &bucket{tokens: l.burst, last: now}
		l.m[key] = b
	} else {
		l.refill(b, now)
	}
	if b.tokens >= 1-epsilon { // epsilon absorbs float rounding of the refill
		b.tokens = math.Max(0, b.tokens-1)
		return true, 0
	}
	return false, l.wait(1 - b.tokens)
}

func (l *Limiter) refill(b *bucket, now time.Time) {
	if d := now.Sub(b.last); d > 0 {
		b.tokens = math.Min(l.burst, b.tokens+d.Seconds()*l.rate)
		b.last = now
	} else if d < 0 { // clock stepped back: do not mint tokens
		b.last = now
	}
}

func (l *Limiter) wait(missing float64) time.Duration {
	return time.Duration(math.Ceil(missing / l.rate * float64(time.Second)))
}

// Sweep removes every bucket that has refilled completely: it behaves exactly
// like a missing key, so dropping it changes nothing. It returns the number
// removed.
func (l *Limiter) Sweep() int {
	now := l.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sweepLocked(now)
}

func (l *Limiter) sweepLocked(now time.Time) int {
	n := 0
	for k, b := range l.m {
		l.refill(b, now)
		if b.tokens >= l.burst {
			delete(l.m, k)
			n++
		}
	}
	return n
}

// Len reports how many keys are tracked.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.m)
}

// Run sweeps every interval until ctx is done. Start it in a goroutine.
func (l *Limiter) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.Sweep()
		}
	}
}
