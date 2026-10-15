package claude

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

const (
	// UsageMinInterval is the shortest gap between usage reads of one access token.
	UsageMinInterval = 3 * time.Minute
	// UsageForceMinInterval is the shortest gap when the user forces a refresh.
	UsageForceMinInterval = 30 * time.Second
	// RateLimitBackoffStart is the first wait after a 429 that has no Retry-After.
	RateLimitBackoffStart = 5 * time.Minute
	// RateLimitBackoffMax caps the exponential backoff.
	RateLimitBackoffMax = time.Hour
)

// UsageThrottle spaces usage reads. It does not refresh tokens: a 429 is a
// wait, never a reason to rotate the access token. Keys are hashes of the
// token so a dump of the state is not the token itself.
type UsageThrottle struct {
	mu    sync.Mutex
	state map[string]throttleState
}

type throttleState struct {
	lastFetch    time.Time
	backoff      time.Duration
	blockedUntil time.Time
}

// Allow reports whether a usage read may start. force lowers the ordinary
// gap from 3 minutes to 30 seconds. A 429 block is never bypassed.
func (t *UsageThrottle) Allow(token string, now time.Time, force bool) (time.Duration, bool) {
	if stringsTrim(token) == "" {
		return 0, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.lookup(token)
	until := time.Time{}
	if st.blockedUntil.After(now) {
		until = st.blockedUntil
	}
	gap := UsageMinInterval
	if force {
		gap = UsageForceMinInterval
	}
	if !st.lastFetch.IsZero() {
		next := st.lastFetch.Add(gap)
		if next.After(until) {
			until = next
		}
	}
	if until.After(now) {
		return until.Sub(now), false
	}
	return 0, true
}

// Blocked reports whether a 429 still holds the token back, as opposed to
// the ordinary spacing between reads.
func (t *UsageThrottle) Blocked(token string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lookup(token).blockedUntil.After(now)
}

// Commit records that a usage request was sent.
func (t *UsageThrottle) Commit(token string, now time.Time) {
	if stringsTrim(token) == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.lookup(token)
	st.lastFetch = now
	t.state[tokenKey(token)] = st
}

// ObserveRateLimit records a 429. A Retry-After header sets the wait.
// Without one, the wait starts at 5 minutes and doubles, capped at 1 hour.
// The returned duration is how long the caller should wait.
func (t *UsageThrottle) ObserveRateLimit(token string, now time.Time, retryAfter time.Duration, hasRetryAfter bool) time.Duration {
	if stringsTrim(token) == "" {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.lookup(token)
	wait := retryAfter
	if hasRetryAfter {
		if wait < 0 {
			wait = 0
		}
	} else {
		wait = nextBackoff(st.backoff)
		st.backoff = wait
	}
	until := now.Add(wait)
	if until.After(st.blockedUntil) {
		st.blockedUntil = until
	}
	t.state[tokenKey(token)] = st
	return wait
}

func (t *UsageThrottle) lookup(token string) throttleState {
	if t.state == nil {
		t.state = map[string]throttleState{}
	}
	return t.state[tokenKey(token)]
}

func nextBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return RateLimitBackoffStart
	}
	next := current * 2
	if next > RateLimitBackoffMax {
		return RateLimitBackoffMax
	}
	return next
}

func tokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func stringsTrim(s string) string {
	i := 0
	j := len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n') {
		j--
	}
	return s[i:j]
}
