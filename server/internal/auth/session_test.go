package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"ngajio/server/internal/store"
)

func TestValidToken(t *testing.T) {
	tok, _, _ := store.NewToken()
	if !ValidToken(tok) {
		t.Fatalf("a fresh token must be valid: %q", tok)
	}
	for _, ok := range []string{strings.Repeat("A", 43), strings.Repeat("-", 43), strings.Repeat("_", 43), strings.Repeat("z9", 21) + "Z"} {
		if !ValidToken(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "a", strings.Repeat("A", 42), strings.Repeat("A", 44), strings.Repeat("A", 42) + "+",
		strings.Repeat("A", 42) + "/", strings.Repeat("A", 42) + "=", strings.Repeat("A", 42) + " ", strings.Repeat("A", 42) + "\n",
		strings.Repeat("A", 42) + "\x00", strings.Repeat("A", 42) + "é", strings.Repeat("é", 21) + "A"} {
		if ValidToken(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestSlideTarget(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	day := 24 * time.Hour
	ss := func(left, sinceSeen time.Duration) store.Session {
		return store.Session{ExpiresAt: now.Add(left).Unix(), LastSeen: now.Add(-sinceSeen).Unix()}
	}
	for _, tc := range []struct {
		name  string
		s     store.Session
		slide bool
	}{
		{"fresh session", ss(30*day, 0), false},
		{"exactly 25 days left", ss(25*day, 48*time.Hour), false},
		{"just under 25 days", ss(25*day-time.Second, 48*time.Hour), true},
		{"under 25 days, written 23 h ago", ss(20*day, 23*time.Hour), false},
		{"under 25 days, written 24 h ago", ss(20*day, 24*time.Hour), true},
		{"1 minute left", ss(time.Minute, 10*day), true},
	} {
		exp, got := slideTarget(tc.s, now)
		if got != tc.slide || (got && !exp.Equal(now.Add(store.SessionTTL))) {
			t.Errorf("%s: got %v %v", tc.name, got, exp)
		}
	}
}

func TestSessionTokensAreRandomAndHashed(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		_, c := h.loginOK("s", "a@example.com", "N")
		if !ValidToken(c) || seen[c] {
			t.Fatalf("bad or repeated token %q", c)
		}
		seen[c] = true
		if _, _, found := h.sessionRow(c); !found {
			t.Fatal("hash of the token is not stored")
		}
	}
	var n int
	h.st.Reader().QueryRow(`SELECT count(*) FROM sessions WHERE length(token_hash)<>32`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d rows with a non-sha256 key", n)
	}
}

func TestCleanupDeletesOnlyExpired(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, old1 := h.loginOK("s1", "a@example.com", "A")
	_, old2 := h.loginOK("s2", "b@example.com", "B")
	h.clock.Add(29 * 24 * time.Hour)
	_, fresh := h.loginOK("s3", "c@example.com", "C")
	h.clock.Add(2 * 24 * time.Hour) // old ones are now 1 day past expiry, fresh has 28 days left
	n, err := h.a.Cleanup(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("cleanup: %d %v", n, err)
	}
	for _, c := range []string{old1, old2} {
		if _, _, found := h.sessionRow(c); found {
			t.Error("expired session survived")
		}
	}
	if _, _, found := h.sessionRow(fresh); !found {
		t.Error("valid session deleted")
	}
	if n, _ := h.a.Cleanup(context.Background()); n != 0 {
		t.Errorf("second cleanup removed %d", n)
	}
	if h.count(`SELECT count(*) FROM users`) != 3 {
		t.Error("cleanup must not touch users")
	}
}

func TestRunCleanupTicksAndStops(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c := h.loginOK("s1", "a@example.com", "A")
	h.clock.Add(31 * 24 * time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.a.RunCleanup(ctx, 5*time.Millisecond); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, _, found := h.sessionRow(c); !found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expired session not removed by RunCleanup")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunCleanup did not stop with its context")
	}
}

func TestRunCleanupStopsBeforeFirstTick(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.a.RunCleanup(ctx, CleanupInterval); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunCleanup ignored cancel")
	}
	if CleanupInterval != time.Hour {
		t.Fatalf("interval %v", CleanupInterval)
	}
}

func TestStartSessionUsesInjectedClock(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c := h.loginOK("s", "a@example.com", "N")
	exp, seen, _ := h.sessionRow(c)
	if seen != h.clock.Now().Unix() || exp != h.clock.Now().Add(store.SessionTTL).Unix() {
		t.Fatalf("exp=%d seen=%d now=%d", exp, seen, h.clock.Now().Unix())
	}
}
