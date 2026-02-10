package auth

import (
	"sync"
	"time"
)

// ipRecord tracks login attempts for a single IP address.
type ipRecord struct {
	attempts  int
	firstSeen time.Time
}

// RateLimiter is an in-memory, IP-based rate limiter for login attempts.
type RateLimiter struct {
	mu          sync.Mutex
	ips         map[string]*ipRecord
	maxAttempts int
	window      time.Duration
	staleAfter  time.Duration
	now         func() time.Time // for testing
}

// NewRateLimiter creates a rate limiter that allows maxAttempts requests per
// IP within the given window. Stale entries (older than 5× the window) are
// purged automatically on every call to Allow.
func NewRateLimiter(maxAttempts int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		ips:         make(map[string]*ipRecord),
		maxAttempts: maxAttempts,
		window:      window,
		staleAfter:  5 * time.Minute,
		now:         time.Now,
	}
}

// Allow checks whether the given IP is permitted to make a login attempt.
// If the limit has been exceeded it returns false and the duration the caller
// should wait before retrying.
func (rl *RateLimiter) Allow(ip string) (allowed bool, retryAfter time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	rl.cleanup(now)

	rec, ok := rl.ips[ip]
	if !ok {
		rl.ips[ip] = &ipRecord{attempts: 1, firstSeen: now}
		return true, 0
	}

	// If the window has elapsed, reset the record.
	elapsed := now.Sub(rec.firstSeen)
	if elapsed >= rl.window {
		rec.attempts = 1
		rec.firstSeen = now
		return true, 0
	}

	// Within the window — check the count.
	if rec.attempts < rl.maxAttempts {
		rec.attempts++
		return true, 0
	}

	// Limit exceeded; tell the caller how long to wait.
	return false, rl.window - elapsed
}

// Reset removes all tracking state for the given IP. Call this after a
// successful login so the user isn't penalised on subsequent attempts.
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.ips, ip)
}

// cleanup removes entries whose window started more than staleAfter ago.
// Must be called with rl.mu held.
func (rl *RateLimiter) cleanup(now time.Time) {
	for ip, rec := range rl.ips {
		if now.Sub(rec.firstSeen) > rl.staleAfter {
			delete(rl.ips, ip)
		}
	}
}
