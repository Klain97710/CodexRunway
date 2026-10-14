package export

import (
	"reflect"
	"time"

	"github.com/licoy/quosparr/internal/alerts"
)

// Verdict is the reset-today answer at one instant (the Swift
// RateLimitResetTodayVerdictPresentation, reduced to what the widget stores).
type Verdict struct {
	Reason      VerdictReason
	ShowsYes    bool
	ResetType   *alerts.ResetType
	IsScheduled bool
	// ScheduleBasis is the basis of the scheduled reset the verdict is about,
	// meaningful when IsScheduled.
	ScheduleBasis *alerts.ScheduleBasis
	Percent       *int
	Band          *ConfidenceBand
}

// NextReset is the earliest open scheduled reset (nextScheduledResetSummary).
type NextReset struct {
	StartAt      time.Time
	PendingUntil time.Time
	ResetType    alerts.ResetType
	// ScheduleBasis is contextual only when every merged schedule was.
	ScheduleBasis *alerts.ScheduleBasis
}

// ResetTodaySource is the immutable reset-today snapshot with its calendar
// bound in. The verdict rules (RateLimitResetToday*.swift) are not part of this
// package; whoever ports them implements this interface.
type ResetTodaySource interface {
	// Verdict is verdictPresentation(now:).
	Verdict(now time.Time) Verdict
	// NextScheduled is nextScheduledResetSummary(now:), nil when nothing is
	// scheduled.
	NextScheduled(now time.Time) *NextReset
	// TransitionDates is nextVerdictTransitionDates(now:): the instants after
	// now at which the verdict can change.
	TransitionDates(now time.Time) []time.Time
	FetchedAt() time.Time
	LastSuccessfulCheckAt() *time.Time
}

// BuildResetToday ports makeWidgetResetTodaySnapshot. The first entry is the
// verdict now (stamped min(fetchedAt, now)); each later entry is the verdict at
// a transition date, rounded up to the next whole second because the file only
// has second precision. Entries that look the same as their predecessor are
// dropped.
func BuildResetToday(src ResetTodaySource, now time.Time) ResetTodaySnapshot {
	anchor := src.FetchedAt()
	if now.Before(anchor) {
		anchor = now
	}
	current := timelineEntry(src, anchor, now)
	timeline := []ResetTodayEntry{current}
	for _, raw := range src.TransitionDates(now) {
		date := CeilSecond(raw)
		entry := timelineEntry(src, date, date)
		if !samePresentation(timeline[len(timeline)-1], entry) {
			timeline = append(timeline, entry)
		}
	}
	return ResetTodaySnapshot{
		State:                  current.State,
		ResetType:              current.ResetType,
		NextScheduledAt:        current.NextScheduledAt,
		NextScheduledResetType: current.NextScheduledResetType,
		LastSuccessfulCheckAt:  src.LastSuccessfulCheckAt(),
		FetchedAt:              src.FetchedAt(),
		ConfidencePercent:      current.ConfidencePercent,
		ConfidenceBand:         current.ConfidenceBand,
		Timeline:               timeline,
	}
}

// CeilSecond rounds t up to a whole second.
func CeilSecond(t time.Time) time.Time {
	if t.Nanosecond() == 0 {
		return t
	}
	return time.Unix(t.Unix()+1, 0).UTC()
}

func timelineEntry(src ResetTodaySource, effectiveAt, evaluatedAt time.Time) ResetTodayEntry {
	v := src.Verdict(evaluatedAt)
	state := ResetNo
	switch {
	case v.Reason == ReasonUnavailable:
		state = ResetUnknown
	case v.ShowsYes:
		state = ResetYes
	}
	var next *NextReset
	if v.Reason != ReasonUnavailable {
		next = src.NextScheduled(evaluatedAt)
	}
	var timerTarget *time.Time
	if next != nil {
		target := next.PendingUntil
		if next.StartAt.After(evaluatedAt) {
			target = next.StartAt
		}
		timerTarget = &target
	}
	e := ResetTodayEntry{
		EffectiveAt:       effectiveAt,
		Reason:            v.Reason,
		State:             state,
		ResetType:         v.ResetType,
		NextScheduledAt:   timerTarget,
		ConfidencePercent: v.Percent,
		ConfidenceBand:    v.Band,
	}
	if timerTarget != nil {
		rt := next.ResetType
		e.NextScheduledResetType = &rt
	}
	switch {
	case v.IsScheduled:
		e.ScheduleBasis = v.ScheduleBasis
	case next != nil:
		e.ScheduleBasis = next.ScheduleBasis
	}
	return e
}

// samePresentation compares everything but the effective time.
func samePresentation(a, b ResetTodayEntry) bool {
	return a.Reason == b.Reason && a.State == b.State &&
		reflect.DeepEqual(a.ResetType, b.ResetType) &&
		sameTime(a.NextScheduledAt, b.NextScheduledAt) &&
		reflect.DeepEqual(a.NextScheduledResetType, b.NextScheduledResetType) &&
		reflect.DeepEqual(a.ScheduleBasis, b.ScheduleBasis) &&
		reflect.DeepEqual(a.ConfidencePercent, b.ConfidencePercent) &&
		reflect.DeepEqual(a.ConfidenceBand, b.ConfidenceBand)
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
