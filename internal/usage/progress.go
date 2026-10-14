package usage

import (
	"sync"
	"time"
)

// Phase is a coarse stage of a cost scan.
type Phase string

// Scan phases.
const (
	PhaseIdle            Phase = "idle"
	PhasePreparing       Phase = "preparing"
	PhaseRefreshingIndex Phase = "refreshingIndex"
	PhaseAggregating     Phase = "aggregating"
	PhaseFetchingOnline  Phase = "fetchingOnline"
	PhaseFinished        Phase = "finished"
	PhaseFailed          Phase = "failed"
)

// Progress is a small, copyable snapshot of scan progress: counts and short
// labels only, so it is cheap to hand to a UI thread.
type Progress struct {
	Phase          Phase
	CompletedUnits int
	// TotalUnits is nil when the total is not known.
	TotalUnits *int
	// Detail names the item in flight, such as the file being indexed.
	Detail  string
	Message string
}

// Fraction returns the completed share in [0, 1], or ok=false without a total.
func (p Progress) Fraction() (fraction float64, ok bool) {
	if p.TotalUnits == nil || *p.TotalUnits <= 0 {
		return 0, false
	}
	f := float64(p.CompletedUnits) / float64(*p.TotalUnits)
	return min(1, max(0, f)), true
}

// IsActive reports whether the phase is scan work.
func (p Progress) IsActive() bool {
	switch p.Phase {
	case PhasePreparing, PhaseRefreshingIndex, PhaseAggregating, PhaseFetchingOnline:
		return true
	}
	return false
}

func intPtr(v int) *int { return &v }

// Progress constructors.
func ProgressIdle() Progress      { return Progress{Phase: PhaseIdle} }
func ProgressPreparing() Progress { return Progress{Phase: PhasePreparing} }
func ProgressRefreshingIndex(completed, total int, currentFile string) Progress {
	return Progress{Phase: PhaseRefreshingIndex, CompletedUnits: completed, TotalUnits: intPtr(total), Detail: currentFile}
}
func ProgressAggregating(completed, total int) Progress {
	return Progress{Phase: PhaseAggregating, CompletedUnits: completed, TotalUnits: intPtr(total)}
}
func ProgressFetchingOnline() Progress { return Progress{Phase: PhaseFetchingOnline} }
func ProgressFinished() Progress       { return Progress{Phase: PhaseFinished} }
func ProgressFailed(message string) Progress {
	return Progress{Phase: PhaseFailed, Message: message}
}

// ProgressReporter is a goroutine-safe progress sink. High-frequency index
// updates are throttled to one per minimum interval; everything else, and any
// report marked force, is published immediately. A nil *ProgressReporter
// discards reports.
type ProgressReporter struct {
	mu       sync.Mutex
	handler  func(Progress)
	last     time.Time
	interval time.Duration
	now      func() time.Time
}

// NewProgressReporter returns a reporter that calls onProgress (which may be
// nil) at most once per minimumInterval for index updates. A zero interval
// means the default of 100 ms; use a negative one to disable throttling.
func NewProgressReporter(minimumInterval time.Duration, onProgress func(Progress)) *ProgressReporter {
	if minimumInterval == 0 {
		minimumInterval = 100 * time.Millisecond
	}
	return &ProgressReporter{handler: onProgress, interval: minimumInterval, now: time.Now}
}

// SetHandler replaces the callback.
func (r *ProgressReporter) SetHandler(onProgress func(Progress)) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.handler = onProgress
	r.mu.Unlock()
}

// Report publishes p, subject to throttling unless force is set. The handler
// is called without the reporter's lock held.
func (r *ProgressReporter) Report(p Progress, force bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	handler := r.handler
	now := r.now()
	publish := force || !p.IsActive() || p.Phase != PhaseRefreshingIndex ||
		r.interval < 0 || now.Sub(r.last) >= r.interval
	if publish {
		r.last = now
	}
	r.mu.Unlock()
	if publish && handler != nil {
		handler(p)
	}
}
