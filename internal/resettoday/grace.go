package resettoday

import "time"

// mergedScheduleCandidates are the scheduled events of the feed, the timeline's
// next schedule and the grace schedules, one per post ID; a later source
// replaces an earlier one in place.
func (s *Snapshot) mergedScheduleCandidates() []Event {
	var merged []Event
	index := map[string]int{}
	insert := func(e Event) {
		if i, ok := index[e.Source.PostID]; ok {
			merged[i] = e
			return
		}
		index[e.Source.PostID] = len(merged)
		merged = append(merged, e)
	}
	for _, e := range s.Events {
		if e.Kind == KindResetScheduled {
			insert(e)
		}
	}
	if s.ResetTimeline != nil && s.ResetTimeline.NextSchedule != nil {
		insert(*s.ResetTimeline.NextSchedule)
	}
	for _, e := range s.GraceSchedules {
		insert(e)
	}
	return merged
}

// hasCompletion reports whether a completion of the schedule happened up to
// now: a completed event of a covering type and compatible scope announced
// after the schedule, or a manual completion.
func (s *Snapshot) hasCompletion(schedule Event, w ScheduleWindow, now time.Time) bool {
	suppressed := s.suppressed()
	lower := completionLowerBound(schedule, w)
	for _, e := range s.Events {
		if e.Kind != KindResetCompleted || suppressed[e.Source.PostID] ||
			e.AnnouncedAt.Before(schedule.AnnouncedAt) ||
			!e.ResetType.Includes(schedule.ResetType) ||
			!scopesAreCompatible(schedule.Scope, e.Scope) {
			continue
		}
		at := e.AnnouncedAt
		if e.EffectiveAt != nil {
			at = *e.EffectiveAt
		}
		if !lower.After(at) && !at.After(now) {
			return true
		}
	}
	if s.ResetTimeline == nil {
		return false
	}
	for _, m := range s.ResetTimeline.ManualCompletions {
		typeMatches := schedule.ResetType == ResetGlobalAndBanked || m.ResetType().Includes(schedule.ResetType)
		if typeMatches && !schedule.AnnouncedAt.After(m.CompletedAt) &&
			!lower.After(m.CompletedAt) && !m.CompletedAt.After(now) {
			return true
		}
	}
	return false
}

// completionLowerBound: a range accepts completions from its start; a point
// accepts them from 30 minutes before, or the start of its source-calendar
// day if that is earlier.
func completionLowerBound(schedule Event, w ScheduleWindow) time.Time {
	if w.IsRange || schedule.EffectiveAt == nil {
		return w.StartAt
	}
	at := *schedule.EffectiveAt
	day := startOfDay(at, sourceLocation())
	if early := at.Add(-30 * time.Minute); early.Before(day) {
		return early
	}
	return day
}

func scopesAreCompatible(a, b Scope) bool {
	return axesAreCompatible(a.Plans, b.Plans, "all", "unknown") &&
		axesAreCompatible(a.Windows, b.Windows, "unknown")
}

func axesAreCompatible(left, right []string, wildcards ...string) bool {
	if len(left) == 0 || len(right) == 0 {
		return true
	}
	l, r := stringSet(left), stringSet(right)
	for _, w := range wildcards {
		if l[w] || r[w] {
			return true
		}
	}
	for v := range l {
		if r[v] {
			return true
		}
	}
	return false
}
