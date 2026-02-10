package auth

import (
	"sync"
	"testing"
	"time"
)

func newTestLimiter() (*RateLimiter, *time.Time) {
	rl := NewRateLimiter(5, time.Minute)
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	rl.now = func() time.Time { return now }
	return rl, &now
}

func TestAllow_UnderLimit(t *testing.T) {
	rl, _ := newTestLimiter()

	for i := 0; i < 5; i++ {
		ok, retry := rl.Allow("1.2.3.4")
		if !ok {
			t.Fatalf("attempt %d: expected allowed, got blocked (retryAfter=%s)", i+1, retry)
		}
	}
}

func TestAllow_ExceedsLimit(t *testing.T) {
	rl, _ := newTestLimiter()

	for i := 0; i < 5; i++ {
		rl.Allow("1.2.3.4")
	}

	ok, retry := rl.Allow("1.2.3.4")
	if ok {
		t.Fatal("expected blocked after 5 attempts")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("expected retryAfter in (0, 1m], got %s", retry)
	}
}

func TestAllow_RetryAfterDecreases(t *testing.T) {
	rl, now := newTestLimiter()

	for i := 0; i < 5; i++ {
		rl.Allow("1.2.3.4")
	}

	// Advance 30s into the 1-minute window.
	*now = now.Add(30 * time.Second)

	ok, retry := rl.Allow("1.2.3.4")
	if ok {
		t.Fatal("expected blocked")
	}
	if retry != 30*time.Second {
		t.Fatalf("expected retryAfter=30s, got %s", retry)
	}
}

func TestAllow_WindowResets(t *testing.T) {
	rl, now := newTestLimiter()

	for i := 0; i < 5; i++ {
		rl.Allow("1.2.3.4")
	}

	// Advance past the window.
	*now = now.Add(61 * time.Second)

	ok, _ := rl.Allow("1.2.3.4")
	if !ok {
		t.Fatal("expected allowed after window reset")
	}
}

func TestReset(t *testing.T) {
	rl, _ := newTestLimiter()

	for i := 0; i < 5; i++ {
		rl.Allow("1.2.3.4")
	}

	rl.Reset("1.2.3.4")

	ok, _ := rl.Allow("1.2.3.4")
	if !ok {
		t.Fatal("expected allowed after reset")
	}
}

func TestAllow_DifferentIPs(t *testing.T) {
	rl, _ := newTestLimiter()

	for i := 0; i < 5; i++ {
		rl.Allow("1.2.3.4")
	}

	ok, _ := rl.Allow("5.6.7.8")
	if !ok {
		t.Fatal("different IP should not be affected")
	}
}

func TestCleanup_StaleEntries(t *testing.T) {
	rl, now := newTestLimiter()

	rl.Allow("old-ip")

	// Advance past staleAfter (5 minutes).
	*now = now.Add(6 * time.Minute)

	// Trigger cleanup via Allow on another IP.
	rl.Allow("new-ip")

	rl.mu.Lock()
	_, exists := rl.ips["old-ip"]
	rl.mu.Unlock()

	if exists {
		t.Fatal("stale entry should have been cleaned up")
	}
}

func TestAllow_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.Allow("1.2.3.4")
		}()
	}
	wg.Wait()

	// No race/panic is the success condition.
	// Also verify we didn't allow more than 5.
	rl.mu.Lock()
	rec := rl.ips["1.2.3.4"]
	rl.mu.Unlock()

	if rec.attempts > 5 {
		t.Fatalf("expected attempts <= 5, got %d", rec.attempts)
	}
}
