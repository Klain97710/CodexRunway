package resettoday

import (
	"context"
	"sync"
	"time"

	"github.com/licoy/quosparr/internal/alerts"
)

// RefreshState is what the UI renders for the reset-today section.
type RefreshState struct {
	// Snapshot is the last successful feed; kept across failures.
	Snapshot *Snapshot
	// Err is the last refresh failure; cleared by the next success.
	Err error
	// Refreshing is true while a fetch is in flight (the section spinner).
	Refreshing bool
	// LastFetch is when the last attempt, successful or not, finished.
	LastFetch time.Time
}

// Refresher owns the independent refresh cycle of the status feed: fetch on
// demand or when the interval has elapsed, keep the last snapshot through a
// failure, and decide the reset alerts between and after fetches.
type Refresher struct {
	Fetch func(ctx context.Context, now time.Time, loc *time.Location) (*Snapshot, error)
	// Loc is the viewer's local-day calendar; nil means time.Local.
	Loc *time.Location
	// Now is the clock; nil means time.Now.
	Now func() time.Time
	// Interval is the user's refresh interval
	// (rateLimitResetTodayRefreshIntervalSeconds); required.
	Interval func() time.Duration
	// Needed says whether anything shows reset-today data (the section or the
	// widget); nil means always.
	Needed func() bool
	// AlertsEnabled is the user's reset-today alert switch; nil means on.
	AlertsEnabled func() bool
	// Deliver receives the alert candidates when alerts are enabled and there
	// are some; the caller dedupes them (alerts.Store.Unseen) and notifies.
	Deliver func([]alerts.Alert)
	// OnChange is called after every state change, never concurrently.
	OnChange func(RefreshState)

	emitMu sync.Mutex
	mu     sync.Mutex
	state  RefreshState
}

func (r *Refresher) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Refresher) needed() bool { return r.Needed == nil || r.Needed() }

// State returns a copy of the current state.
func (r *Refresher) State() RefreshState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (r *Refresher) emit() {
	if r.OnChange == nil {
		return
	}
	r.emitMu.Lock()
	defer r.emitMu.Unlock()
	r.OnChange(r.State())
}

func (r *Refresher) deliver(list []alerts.Alert) {
	if len(list) == 0 || r.Deliver == nil || (r.AlertsEnabled != nil && !r.AlertsEnabled()) {
		return
	}
	r.Deliver(list)
}

// Refresh fetches the feed. A forced refresh ignores the interval; an unforced
// one is skipped while the last attempt is younger than the interval. Nothing
// happens when no data is needed or a fetch is already running. It reports
// whether a fetch ran.
func (r *Refresher) Refresh(ctx context.Context, force bool) bool {
	if !r.needed() {
		return false
	}
	r.mu.Lock()
	if r.state.Refreshing {
		r.mu.Unlock()
		return false
	}
	if !force && !r.state.LastFetch.IsZero() && r.now().Sub(r.state.LastFetch) < r.Interval() {
		r.mu.Unlock()
		return false
	}
	r.state.Refreshing = true
	r.mu.Unlock()
	r.emit()

	snapshot, err := r.Fetch(ctx, r.now(), r.Loc)

	now := r.now()
	r.mu.Lock()
	previous := r.state.Snapshot
	r.state.Refreshing = false
	r.state.LastFetch = now
	var list []alerts.Alert
	if err == nil {
		r.state.Err = nil
		r.state.Snapshot = snapshot
		list = Alerts(previous, snapshot, now, r.Loc)
	} else {
		r.state.Err = err
		if previous != nil {
			// Re-apply the kept snapshot: nothing is new, so only the approaching
			// schedule can alert.
			list = Alerts(previous, previous, now, r.Loc)
		}
	}
	r.mu.Unlock()
	r.deliver(list)
	r.emit()
	return true
}

// Tick runs on the app's timer: it re-checks approaching schedules against the
// last snapshot, then refreshes when the interval is due.
func (r *Refresher) Tick(ctx context.Context, now time.Time) {
	r.mu.Lock()
	current := r.state.Snapshot
	r.mu.Unlock()
	if current != nil {
		r.deliver(Alerts(current, current, now, r.Loc))
	}
	if r.needed() {
		r.Refresh(ctx, false)
	}
}
