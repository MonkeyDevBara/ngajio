package limit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newLim(rate float64, burst int) (*Limiter, *clock) {
	c := &clock{t: time.Unix(1_700_000_000, 0)}
	l := New(rate, burst)
	l.Now = c.now
	return l, c
}

func TestBurstThenDeny(t *testing.T) {
	l, _ := newLim(10.0/60, 10)
	for i := 0; i < 10; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d denied", i+1)
		}
	}
	ok, ra := l.Allow("a")
	if ok || ra <= 0 || ra > 7*time.Second {
		t.Fatalf("11th: ok=%v retryAfter=%v", ok, ra)
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("other key must be unaffected")
	}
}

func TestRefill(t *testing.T) {
	l, c := newLim(10.0/60, 10)
	for i := 0; i < 10; i++ {
		l.Allow("a")
	}
	c.add(5 * time.Second) // under one token (6 s per token)
	if ok, ra := l.Allow("a"); ok || ra > time.Second+time.Millisecond {
		t.Fatalf("early: ok=%v ra=%v", ok, ra)
	}
	c.add(time.Second)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("token should be back after 6 s")
	}
	c.add(time.Hour) // refill is capped at burst
	n := 0
	for ok, _ := l.Allow("a"); ok; ok, _ = l.Allow("a") {
		n++
	}
	if n != 10 {
		t.Fatalf("after long idle got %d tokens, want burst 10", n)
	}
}

func TestClockStepBackDoesNotMint(t *testing.T) {
	l, c := newLim(1, 2)
	l.Allow("a")
	l.Allow("a")
	c.add(-time.Hour)
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("clock going back must not give tokens")
	}
}

func TestSweepEvictsIdleKeys(t *testing.T) {
	l, c := newLim(1, 3)
	for i := 0; i < 100; i++ {
		l.Allow(fmt.Sprintf("ip%d", i))
	}
	l.Allow("busy")
	l.Allow("busy")
	l.Allow("busy")
	l.Allow("busy") // empty bucket
	if l.Len() != 101 {
		t.Fatalf("len %d", l.Len())
	}
	if n := l.Sweep(); n != 0 {
		t.Fatalf("nothing is refilled yet, swept %d", n)
	}
	c.add(2 * time.Second) // ip keys used 1 of 3 tokens and are full again; busy needs 3 s
	if n := l.Sweep(); n != 100 || l.Len() != 1 {
		t.Fatalf("swept %d, len %d, want 100 and 1", n, l.Len())
	}
	c.add(10 * time.Second)
	l.Sweep()
	if l.Len() != 0 {
		t.Fatalf("len %d after full refill", l.Len())
	}
}

func TestEvictedKeyIsFullAgain(t *testing.T) {
	l, c := newLim(1, 2)
	l.Allow("a")
	l.Allow("a")
	c.add(5 * time.Second)
	l.Sweep()
	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatal("full burst expected after eviction")
		}
	}
}

func TestMaxKeysFailsClosed(t *testing.T) {
	l, c := newLim(1, 1)
	l.max = 5
	for i := 0; i < 5; i++ {
		l.Allow(fmt.Sprint(i))
	}
	if ok, ra := l.Allow("new"); ok || ra <= 0 {
		t.Fatalf("full map must refuse new keys: ok=%v ra=%v", ok, ra)
	}
	if l.Len() != 5 {
		t.Fatalf("len %d", l.Len())
	}
	c.add(2 * time.Second) // buckets refilled: the inline sweep frees room
	if ok, _ := l.Allow("new"); !ok {
		t.Fatal("new key should fit after sweep")
	}
	if l.Len() != 1 {
		t.Fatalf("len %d", l.Len())
	}
}

func TestRunStopsOnContext(t *testing.T) {
	l, _ := newLim(1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { l.Run(ctx, time.Millisecond); close(done) }()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestRunSweeps(t *testing.T) {
	l, c := newLim(1, 1)
	l.Allow("a")
	c.add(5 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go l.Run(ctx, time.Millisecond)
	deadline := time.Now().Add(2 * time.Second)
	for l.Len() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("idle key not evicted by Run")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestConcurrent(t *testing.T) {
	l := New(1000, 50)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				l.Allow(fmt.Sprint(i % 20))
				if i%100 == 0 {
					l.Sweep()
				}
			}
		}()
	}
	wg.Wait()
}

func TestNewPanicsOnBadArgs(t *testing.T) {
	for _, c := range []struct {
		r float64
		b int
	}{{0, 1}, {-1, 1}, {1, 0}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%v,%d) must panic", c.r, c.b)
				}
			}()
			New(c.r, c.b)
		}()
	}
}
