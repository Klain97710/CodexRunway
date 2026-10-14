package resettoday

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/alerts"
)

type fetchQueue struct {
	mu    sync.Mutex
	items []func() (*Snapshot, error)
	calls atomic.Int32
	gate  chan struct{}
}

func (q *fetchQueue) fetch(ctx context.Context, _ time.Time, _ *time.Location) (*Snapshot, error) {
	n := q.calls.Add(1)
	if q.gate != nil {
		select {
		case <-q.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	idx := int(n) - 1
	if idx >= len(q.items) {
		idx = len(q.items) - 1
	}
	return q.items[idx]()
}

func snap(s *Snapshot) func() (*Snapshot, error) { return func() (*Snapshot, error) { return s, nil } }

func newRefresher(q *fetchQueue, now func() time.Time) *Refresher {
	return &Refresher{
		Fetch: q.fetch, Loc: time.UTC, Now: now,
		Interval: func() time.Duration { return 6 * time.Hour },
	}
}

func TestForcedRefreshBypassesIntervalAndShowsSpinner(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	no := NewStateSnapshot(StateNo, now)
	yes := DevMock(MockYes, now, time.UTC)
	q := &fetchQueue{items: []func() (*Snapshot, error){snap(no), snap(yes)}, gate: make(chan struct{})}
	r := newRefresher(q, func() time.Time { return now })

	done := make(chan bool)
	go func() { done <- r.Refresh(context.Background(), true) }()
	waitUntil(t, func() bool { return q.calls.Load() >= 1 })
	if s := r.State(); !s.Refreshing || s.Snapshot != nil {
		t.Fatalf("while fetching: %+v", s)
	}
	q.gate <- struct{}{}
	if !<-done {
		t.Fatal("forced refresh did not run")
	}
	if s := r.State(); s.Refreshing || s.Snapshot == nil || s.Snapshot.State() != StateNo {
		t.Fatalf("after first fetch: %+v", s)
	}

	if r.Refresh(context.Background(), false) || q.calls.Load() != 1 {
		t.Fatal("unforced refresh ignored the interval")
	}

	go func() { done <- r.Refresh(context.Background(), true) }()
	waitUntil(t, func() bool { return q.calls.Load() >= 2 })
	if !r.State().Refreshing {
		t.Error("no spinner on the second forced refresh")
	}
	q.gate <- struct{}{}
	<-done
	if s := r.State(); s.Snapshot.State() != StateYes || q.calls.Load() != 2 {
		t.Errorf("state = %v, calls = %d", s.Snapshot.State(), q.calls.Load())
	}
}

func TestUnforcedRefreshRunsAfterInterval(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	clock := now
	q := &fetchQueue{items: []func() (*Snapshot, error){snap(NewStateSnapshot(StateNo, now))}}
	r := newRefresher(q, func() time.Time { return clock })
	r.Refresh(context.Background(), false)
	clock = now.Add(6*time.Hour - time.Second)
	if r.Refresh(context.Background(), false) {
		t.Error("ran before the interval")
	}
	clock = now.Add(6 * time.Hour)
	if !r.Refresh(context.Background(), false) {
		t.Error("did not run at the interval")
	}
}

func TestHiddenModuleSkipsFetch(t *testing.T) {
	q := &fetchQueue{items: []func() (*Snapshot, error){snap(nil)}}
	r := newRefresher(q, time.Now)
	r.Needed = func() bool { return false }
	if r.Refresh(context.Background(), true) || q.calls.Load() != 0 || r.State().Refreshing {
		t.Fatal("fetched while hidden")
	}
}

func TestSingleFlight(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	q := &fetchQueue{items: []func() (*Snapshot, error){snap(NewStateSnapshot(StateNo, now))}, gate: make(chan struct{})}
	r := newRefresher(q, func() time.Time { return now })
	done := make(chan bool)
	go func() { done <- r.Refresh(context.Background(), true) }()
	waitUntil(t, func() bool { return r.State().Refreshing })
	if r.Refresh(context.Background(), true) {
		t.Error("second concurrent fetch started")
	}
	q.gate <- struct{}{}
	<-done
}

func TestRefreshFailureKeepsSnapshotAndErrorUntilRecovery(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	first := NewStateSnapshot(StateNo, now)
	recovered := DevMock(MockYes, now, time.UTC)
	q := &fetchQueue{items: []func() (*Snapshot, error){
		snap(first),
		func() (*Snapshot, error) { return nil, errors.New("cannot connect") },
		snap(recovered),
	}}
	r := newRefresher(q, func() time.Time { return now })
	ctx := context.Background()

	r.Refresh(ctx, true)
	if r.State().Snapshot != first || r.State().Err != nil {
		t.Fatal("first fetch")
	}
	r.Refresh(ctx, true)
	if s := r.State(); s.Snapshot != first || s.Err == nil {
		t.Fatalf("after failure: %+v", s)
	}
	r.Tick(ctx, now.Add(time.Minute))
	if s := r.State(); s.Snapshot != first || s.Err == nil {
		t.Fatalf("after tick: %+v", s)
	}
	r.Refresh(ctx, true)
	if s := r.State(); s.Snapshot != recovered || s.Err != nil {
		t.Fatalf("after recovery: %+v", s)
	}
}

func TestFirstFetchFailureHasNoSnapshot(t *testing.T) {
	q := &fetchQueue{items: []func() (*Snapshot, error){func() (*Snapshot, error) { return nil, errors.New("down") }}}
	r := newRefresher(q, time.Now)
	r.Refresh(context.Background(), true)
	if s := r.State(); s.Snapshot != nil || s.Err == nil || s.Refreshing {
		t.Fatalf("state = %+v", s)
	}
}

func graceSnapshot(now time.Time, hours float64) *Snapshot {
	s := DevMock(MockNo, now, time.UTC)
	eff := now.Add(-4 * time.Hour)
	handle := "thsottiaux"
	url := "https://x.com/thsottiaux/status/2090000000000000001"
	e := Event{
		Kind: KindResetScheduled, ResetType: ResetGlobal, AnnouncedAt: now.Add(-5 * time.Hour), EffectiveAt: &eff,
		SchedulePrecision: PrecisionDatetime, ScheduleBasis: BasisExplicit, ScheduleGraceHours: &hours,
		Scope:      Scope{Plans: []string{"all"}, Windows: []string{"weekly"}},
		Source:     EventSource{Origin: OriginX, Handle: &handle, PostID: "2090000000000000001", URL: &url},
		Confidence: 0.98, Rationale: "Explicit Codex quota reset schedule.", Text: "A reset is scheduled.",
	}
	s.Events = []Event{e}
	s.GraceSchedules = nil
	if hours > 3 {
		s.GraceSchedules = []Event{e}
	}
	return s
}

func TestRefreshedGraceDurationUpdatesPresentation(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	q := &fetchQueue{items: []func() (*Snapshot, error){
		snap(graceSnapshot(now, 3)), snap(graceSnapshot(now, 24)), snap(graceSnapshot(now, 0)),
	}}
	r := newRefresher(q, func() time.Time { return now })
	for i, want := range []bool{false, true, false} {
		r.Refresh(context.Background(), true)
		p := r.State().Snapshot.VerdictPresentation(now, time.UTC)
		if p.ShowsYes != want || p.IsCompleted {
			t.Errorf("refresh %d: showsYes = %v, want %v (%v)", i, p.ShowsYes, want, p.Reason)
		}
	}
}

func TestRefresherDeliversAlertsOnlyWhenEnabled(t *testing.T) {
	now := at(t, "2026-07-29T12:00:00Z")
	previous := newFeed(now).mustDecode(t)
	current := newFeed(now, newEvent("reset_completed", "2026-07-29T04:09:02Z")).mustDecode(t)
	q := &fetchQueue{items: []func() (*Snapshot, error){snap(previous), snap(current)}}
	var delivered [][]alerts.Alert
	enabled := true
	r := newRefresher(q, func() time.Time { return now })
	r.AlertsEnabled = func() bool { return enabled }
	r.Deliver = func(a []alerts.Alert) { delivered = append(delivered, a) }

	r.Refresh(context.Background(), true) // first load never alerts
	if len(delivered) != 0 {
		t.Fatalf("first load alerted: %+v", delivered)
	}
	r.Refresh(context.Background(), true)
	if len(delivered) != 1 || len(delivered[0]) != 1 || delivered[0][0].Kind != "rateLimitResetDetected" {
		t.Fatalf("delivered = %+v", delivered)
	}

	enabled = false
	q.calls.Store(0)
	q.items = []func() (*Snapshot, error){snap(previous), snap(current)}
	r2 := newRefresher(q, func() time.Time { return now })
	r2.AlertsEnabled = func() bool { return false }
	n := 0
	r2.Deliver = func([]alerts.Alert) { n++ }
	r2.Refresh(context.Background(), true)
	r2.Refresh(context.Background(), true)
	if n != 0 {
		t.Error("delivered while alerts are off")
	}
}

func TestTickAlertsOnApproachingSchedule(t *testing.T) {
	now := at(t, "2026-07-29T12:00:00Z")
	s := newFeed(now, newEvent("reset_scheduled", "2026-07-29T05:44:16.000Z", effective("2026-07-29T12:25:00.000Z"))).mustDecode(t)
	q := &fetchQueue{items: []func() (*Snapshot, error){snap(s)}}
	var got []alerts.Alert
	r := newRefresher(q, func() time.Time { return now })
	r.Deliver = func(a []alerts.Alert) { got = append(got, a...) }
	r.Refresh(context.Background(), true)
	got = nil
	r.Tick(context.Background(), now)
	if len(got) != 1 || got[0].Kind != "rateLimitResetUpcoming" || got[0].Threshold == nil || *got[0].Threshold != 30 {
		t.Fatalf("alerts = %+v", got)
	}
}
