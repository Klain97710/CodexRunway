package app

import (
	"os"
	"time"
)

// latencyBudget scales a wall-clock input budget for slow environments. Shared
// CI runners add scheduling jitter on top of race instrumentation, so a timing
// sample there must not fail a release; local runs keep the real budget. Tests
// proving an event does not wait on I/O also check a completion flag, so the
// headroom never hides a blocking call.
func latencyBudget(d time.Duration) time.Duration {
	f := raceLatencyFactor
	if os.Getenv("CI") != "" {
		f *= 3
	}
	return time.Duration(f) * d
}
