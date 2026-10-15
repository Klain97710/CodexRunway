package claude

import (
	"testing"
	"time"
)

func TestUsageThrottleIntervalsAndBackoff(t *testing.T) {
	th := &UsageThrottle{}
	token := "test-access-token"
	start := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	if _, ok := th.Allow(token, start, false); !ok {
		t.Fatal("first allow")
	}
	th.Commit(token, start)
	if _, ok := th.Allow(token, start.Add(29*time.Second), true); ok {
		t.Fatal("force inside 30s")
	}
	if _, ok := th.Allow(token, start.Add(31*time.Second), true); !ok {
		t.Fatal("force after 30s")
	}
	if _, ok := th.Allow(token, start.Add(2*time.Minute), false); ok {
		t.Fatal("inside 3m")
	}
	if _, ok := th.Allow(token, start.Add(3*time.Minute), false); !ok {
		t.Fatal("after 3m")
	}
	wait := th.ObserveRateLimit(token, start, 0, false)
	if wait != 5*time.Minute {
		t.Fatal(wait)
	}
	if _, ok := th.Allow(token, start.Add(4*time.Minute), true); ok {
		t.Fatal("force bypassed 429")
	}
	wait = th.ObserveRateLimit(token, start, 0, false)
	if wait != 10*time.Minute {
		t.Fatal(wait)
	}
	cur := 10 * time.Minute
	for cur < RateLimitBackoffMax {
		wait = th.ObserveRateLimit(token, start, 0, false)
		cur *= 2
		if cur > RateLimitBackoffMax {
			cur = RateLimitBackoffMax
		}
		if wait != cur {
			t.Fatalf("backoff %s want %s", wait, cur)
		}
	}
	if th.ObserveRateLimit(token, start, 0, false) != time.Hour {
		t.Fatal("cap")
	}
	other := &UsageThrottle{}
	if got := other.ObserveRateLimit(token, start, 12*time.Second, true); got != 12*time.Second {
		t.Fatal(got)
	}
	if _, ok := other.Allow(token, start.Add(11*time.Second), true); ok {
		t.Fatal("retry-after")
	}
}
