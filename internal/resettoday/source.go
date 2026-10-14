package resettoday

import (
	"time"

	"github.com/licoy/quosparr/internal/alerts"
	"github.com/licoy/quosparr/internal/export"
)

// Source binds a Snapshot to the viewer's calendar and implements
// export.ResetTodaySource, so the widget timeline is built by the real
// verdict code.
type Source struct {
	Snapshot *Snapshot
	// Loc is the viewer's local-day calendar; nil means time.Local.
	Loc *time.Location
}

var _ export.ResetTodaySource = Source{}

// NewSource binds snapshot to loc.
func NewSource(snapshot *Snapshot, loc *time.Location) Source {
	return Source{Snapshot: snapshot, Loc: loc}
}

// Verdict implements export.ResetTodaySource.
func (s Source) Verdict(now time.Time) export.Verdict {
	return s.Snapshot.VerdictPresentation(now, s.Loc).ExportVerdict()
}

// NextScheduled implements export.ResetTodaySource.
func (s Source) NextScheduled(now time.Time) *export.NextReset {
	next := s.Snapshot.NextScheduledResetSummary(now)
	if next == nil {
		return nil
	}
	r := &export.NextReset{
		StartAt:      next.Window.StartAt,
		PendingUntil: next.Window.PendingUntil(),
		ResetType:    next.ResetType,
	}
	if next.ScheduleBasis != "" {
		b := next.ScheduleBasis
		r.ScheduleBasis = &b
	}
	return r
}

// TransitionDates implements export.ResetTodaySource.
func (s Source) TransitionDates(now time.Time) []time.Time {
	return s.Snapshot.TransitionDates(now, s.Loc)
}

// FetchedAt implements export.ResetTodaySource.
func (s Source) FetchedAt() time.Time { return s.Snapshot.FetchedAt }

// LastSuccessfulCheckAt implements export.ResetTodaySource.
func (s Source) LastSuccessfulCheckAt() *time.Time { return s.Snapshot.LastSuccessfulCheckAt }

// Widget is makeWidgetResetTodaySnapshot(now:calendar:).
func (s Source) Widget(now time.Time) export.ResetTodaySnapshot {
	return export.BuildResetToday(s, now)
}

// AlertState reduces the snapshot to what the alert decider reads, evaluated
// at now in loc: the verdict reason, the completed events, the manual
// completions and the next scheduled reset. Pass the result of the previous
// fetch (evaluated at the same now) and the new one to alerts.ResetTodayAlerts.
func AlertState(s *Snapshot, now time.Time, loc *time.Location) alerts.ResetTodayState {
	state := alerts.ResetTodayState{
		Unavailable: s.VerdictPresentation(now, loc).Reason == export.ReasonUnavailable,
	}
	for _, e := range s.Events {
		if e.Kind != KindResetCompleted {
			continue
		}
		state.Events = append(state.Events, alerts.Event{
			PostID:    e.Source.PostID,
			Completed: true,
			Date:      *e.resetOccurrenceAt(),
			ResetType: e.ResetType,
		})
	}
	if s.ResetTimeline != nil {
		for _, m := range s.ResetTimeline.ManualCompletions {
			item := alerts.ManualCompletion{
				ID:                   m.ID,
				CompletedAt:          m.CompletedAt,
				VisibleUntil:         m.VisibleUntil,
				RepresentativePostID: m.RepresentativePostID,
				SchedulePostIDs:      m.SchedulePostIDs,
			}
			for _, sch := range m.Schedules {
				item.ScheduleTypes = append(item.ScheduleTypes, sch.ResetType)
			}
			state.ManualCompletions = append(state.ManualCompletions, item)
		}
	}
	if next := s.NextScheduledResetSummary(now); next != nil {
		u := &alerts.UpcomingReset{
			EffectiveAt:    next.Window.StartAt,
			EffectiveUntil: next.Window.EndAt,
			IsRange:        next.Window.IsRange,
			PostID:         next.Event.Source.PostID,
			ResetType:      next.ResetType,
			Confidence:     next.Confidence,
		}
		if next.ScheduleBasis != "" {
			b := next.ScheduleBasis
			u.ScheduleBasis = &b
		}
		state.Upcoming = u
	}
	return state
}

// Alerts is RunwayAlertDecider.rateLimitResetTodayAlerts(previous:current:now:
// calendar:). previous is nil on the first successful load.
func Alerts(previous, current *Snapshot, now time.Time, loc *time.Location) []alerts.Alert {
	var prev *alerts.ResetTodayState
	if previous != nil {
		p := AlertState(previous, now, loc)
		prev = &p
	}
	return alerts.ResetTodayAlerts(prev, AlertState(current, now, loc), now, loc)
}
