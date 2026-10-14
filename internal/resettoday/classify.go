package resettoday

import (
	"sort"
	"time"

	"github.com/licoy/quosparr/internal/export"
)

// UpcomingSummary is the earliest open schedule with same-time reset types
// merged independently of its evidence event.
type UpcomingSummary struct {
	Window        ScheduleWindow
	Event         Event
	ResetType     ResetType
	Confidence    float64
	ScheduleBasis ScheduleBasis // "" when none
}

// EffectiveAt is the window start.
func (u UpcomingSummary) EffectiveAt() time.Time { return u.Window.StartAt }

// EffectiveUntil is the window end.
func (u UpcomingSummary) EffectiveUntil() time.Time { return u.Window.EndAt }

type typedCandidate struct {
	typ ResetType
	c   verdictCandidate
}

type scheduleMatch struct {
	event  Event
	window ScheduleWindow
}

// NextScheduledResetSummary is nextScheduledResetSummary(now:).
func (s *Snapshot) NextScheduledResetSummary(now time.Time) *UpcomingSummary {
	var found []struct {
		typ ResetType
		m   scheduleMatch
	}
	for _, t := range []ResetType{ResetGlobal, ResetBanked} {
		if m := s.upcomingCandidate(t, now); m != nil {
			found = append(found, struct {
				typ ResetType
				m   scheduleMatch
			}{t, *m})
		}
	}
	if len(found) == 0 {
		return nil
	}
	first := found[0].m.window.StartAt
	for _, f := range found {
		if f.m.window.StartAt.Before(first) {
			first = f.m.window.StartAt
		}
	}
	var matching []struct {
		typ ResetType
		m   scheduleMatch
	}
	for _, f := range found {
		if f.m.window.StartAt.Equal(first) {
			matching = append(matching, f)
		}
	}
	types := make([]ResetType, len(matching))
	contextual := true
	confidence := matching[0].m.event.Confidence
	for i, f := range matching {
		types[i] = f.typ
		contextual = contextual && f.m.event.ScheduleBasis == BasisContextualInference
		if f.m.event.Confidence < confidence {
			confidence = f.m.event.Confidence
		}
	}
	resetType, ok := mergeTypes(types)
	if !ok {
		return nil
	}
	basis := matching[0].m.event.ScheduleBasis
	if contextual {
		basis = BasisContextualInference
	}
	return &UpcomingSummary{
		Window:        matching[0].m.window,
		Event:         matching[0].m.event,
		ResetType:     resetType,
		Confidence:    confidence,
		ScheduleBasis: basis,
	}
}

// TransitionDates are the instants after now at which the same immutable
// snapshot can produce a different verdict, sorted and de-duplicated.
func (s *Snapshot) TransitionDates(now time.Time, loc *time.Location) []time.Time {
	loc = locOrLocal(loc)
	var dates []time.Time
	for _, e := range s.mergedScheduleCandidates() {
		w := s.ScheduledResetWindow(e)
		if w == nil {
			continue
		}
		dates = append(dates, w.StartAt, w.PendingUntil())
		if d, ok := e.graceDeadline(w.PendingUntil()); ok {
			dates = append(dates, d)
		}
	}
	for _, e := range s.Events {
		if e.Kind == KindResetCompleted {
			dates = append(dates, *e.resetOccurrenceAt())
		}
	}
	if s.ResetTimeline != nil {
		for _, m := range s.ResetTimeline.ManualCompletions {
			dates = append(dates, m.CompletedAt, m.VisibleUntil.Add(time.Second))
		}
	}
	stale := s.FreshnessAt().Add(StaleAfter + time.Second)
	dates = append(dates, stale)
	midnight := addDay(startOfDay(now, loc), loc)
	for remaining := 64; remaining > 0 && !midnight.After(stale); remaining-- {
		dates = append(dates, midnight)
		midnight = addDay(midnight, loc)
	}
	var out []time.Time
	for _, d := range dates {
		if d.After(now) {
			out = append(out, d)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Before(out[j]) })
	uniq := out[:0]
	for _, d := range out {
		if len(uniq) == 0 || !uniq[len(uniq)-1].Equal(d) {
			uniq = append(uniq, d)
		}
	}
	return uniq
}

type verdictCandidate struct {
	reason        export.VerdictReason
	state         State
	event         *Event
	window        *ScheduleWindow
	confidence    *float64
	completedAt   *time.Time
	scheduleBasis ScheduleBasis
}

var (
	distantPast   = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	distantFuture = time.Date(4001, 1, 1, 0, 0, 0, 0, time.UTC)
)

func (c verdictCandidate) timestamp() time.Time {
	switch c.reason {
	case export.ReasonCompleted:
		if c.completedAt != nil {
			return *c.completedAt
		}
		return distantPast
	case export.ReasonUpcoming:
		if c.window != nil {
			return c.window.StartAt
		}
		return distantFuture
	case export.ReasonGrace, export.ReasonExpiredUnconfirmed:
		if c.window != nil {
			return c.window.PendingUntil()
		}
		return distantPast
	}
	return distantFuture
}

func (s *Snapshot) combinedVerdictCandidate(now time.Time, loc *time.Location) (verdictCandidate, *ResetType) {
	if !s.isHealthy(now) {
		return verdictCandidate{reason: export.ReasonUnavailable, state: StateUnknown}, nil
	}
	byType := []typedCandidate{
		{ResetGlobal, s.verdictCandidate(ResetGlobal, now, loc)},
		{ResetBanked, s.verdictCandidate(ResetBanked, now, loc)},
	}
	var completed []typedCandidate
	for _, t := range byType {
		if t.c.reason == export.ReasonCompleted {
			completed = append(completed, t)
		}
	}
	if len(completed) > 0 {
		return mergeCandidates(completed, export.ReasonCompleted, StateYes)
	}
	var scheduled, live, grace []typedCandidate
	for _, t := range byType {
		switch t.c.reason {
		case export.ReasonUpcoming:
			scheduled = append(scheduled, t)
			live = append(live, t)
		case export.ReasonGrace:
			scheduled = append(scheduled, t)
			grace = append(grace, t)
		case export.ReasonExpiredUnconfirmed:
			scheduled = append(scheduled, t)
		}
	}
	if len(scheduled) == 0 {
		return verdictCandidate{reason: export.ReasonNone, state: StateNo}, nil
	}
	switch {
	case len(live) > 0:
		scheduled = live
	case len(grace) > 0:
		scheduled = grace
	}
	var selected []typedCandidate
	for _, t := range scheduled {
		if t.c.state == StateYes {
			selected = append(selected, t)
		}
	}
	if len(selected) == 0 {
		selected = scheduled
	}
	newestFirst := len(live) == 0 && len(grace) > 0
	sort.SliceStable(selected, func(i, j int) bool {
		if newestFirst {
			return selected[i].c.timestamp().After(selected[j].c.timestamp())
		}
		return selected[i].c.timestamp().Before(selected[j].c.timestamp())
	})
	ts := selected[0].c.timestamp()
	var matching []typedCandidate
	state := StateNo
	for _, t := range selected {
		if t.c.timestamp().Equal(ts) {
			matching = append(matching, t)
			if t.c.state == StateYes {
				state = StateYes
			}
		}
	}
	return mergeCandidates(matching, selected[0].c.reason, state)
}

func mergeCandidates(cands []typedCandidate, reason export.VerdictReason, state State) (verdictCandidate, *ResetType) {
	rep := cands[0]
	for _, c := range cands[1:] {
		if rep.c.timestamp().Before(c.c.timestamp()) {
			rep = c
		}
	}
	var confidence *float64
	var completedAt *time.Time
	types := make([]ResetType, len(cands))
	for i, c := range cands {
		types[i] = c.typ
		if c.c.confidence != nil && (confidence == nil || *c.c.confidence < *confidence) {
			v := *c.c.confidence
			confidence = &v
		}
		if c.c.completedAt != nil && (completedAt == nil || c.c.completedAt.After(*completedAt)) {
			v := *c.c.completedAt
			completedAt = &v
		}
	}
	var basis ScheduleBasis
	if reason == export.ReasonUpcoming || reason == export.ReasonGrace {
		contextual := true
		for _, c := range cands {
			contextual = contextual && c.c.event != nil && c.c.event.ScheduleBasis == BasisContextualInference
		}
		switch {
		case contextual:
			basis = BasisContextualInference
		case cands[0].c.event != nil:
			basis = cands[0].c.event.ScheduleBasis
		}
	}
	merged, _ := mergeTypes(types)
	out := verdictCandidate{
		reason: reason, state: state, event: rep.c.event, window: rep.c.window,
		confidence: confidence, completedAt: completedAt, scheduleBasis: basis,
	}
	if merged == "" {
		return out, nil
	}
	return out, &merged
}

func (s *Snapshot) verdictCandidate(t ResetType, now time.Time, loc *time.Location) verdictCandidate {
	if c := s.completedCandidate(t, now, loc); c != nil {
		return *c
	}
	if up := s.upcomingCandidate(t, now); up != nil {
		state := StateNo
		if isWindow(up.window, now, loc) && up.event.ScheduleBasis != BasisContextualInference {
			state = StateYes
		}
		conf := up.event.Confidence
		ev, w := up.event, up.window
		return verdictCandidate{reason: export.ReasonUpcoming, state: state, event: &ev, window: &w, confidence: &conf}
	}
	if g := s.graceCandidate(t, now); g != nil {
		conf := g.event.Confidence
		ev, w := g.event, g.window
		return verdictCandidate{reason: export.ReasonGrace, state: StateYes, event: &ev, window: &w, confidence: &conf}
	}
	for _, e := range s.mergedScheduleCandidates() {
		if e.Kind != KindResetScheduled || !e.ResetType.Includes(t) {
			continue
		}
		w := s.ScheduledResetWindow(e)
		if w == nil {
			continue
		}
		if p := w.PendingUntil(); !p.After(now) && sameDay(p, now, loc) {
			conf := e.Confidence
			ev := e
			return verdictCandidate{reason: export.ReasonExpiredUnconfirmed, state: StateNo, event: &ev, window: w, confidence: &conf}
		}
	}
	return verdictCandidate{reason: export.ReasonNone, state: StateNo}
}

func (s *Snapshot) completedCandidate(t ResetType, now time.Time, loc *time.Location) *verdictCandidate {
	type completedEvent struct {
		event Event
		at    time.Time
	}
	var events []completedEvent
	for _, e := range s.Events {
		if e.Kind != KindResetCompleted || !e.ResetType.Includes(t) {
			continue
		}
		at := e.AnnouncedAt
		if e.EffectiveAt != nil {
			at = *e.EffectiveAt
		}
		if at.After(now) || !sameDay(at, now, loc) {
			continue
		}
		events = append(events, completedEvent{e, at})
	}
	var manual []ManualCompletion
	for _, m := range s.VisibleManualCompletions(now) {
		if m.ResetType().Includes(t) && !m.CompletedAt.After(now) && sameDay(m.CompletedAt, now, loc) {
			manual = append(manual, m)
		}
	}
	if len(events) > 0 {
		evidence := events[0]
		for _, c := range events[1:] {
			if !c.event.AnnouncedAt.Equal(evidence.event.AnnouncedAt) {
				if evidence.event.AnnouncedAt.Before(c.event.AnnouncedAt) {
					evidence = c
				}
			} else if evidence.event.Source.PostID < c.event.Source.PostID {
				evidence = c
			}
		}
		completedAt := events[0].at
		for _, c := range events {
			if c.at.After(completedAt) {
				completedAt = c.at
			}
		}
		for _, m := range manual {
			if m.CompletedAt.After(completedAt) {
				completedAt = m.CompletedAt
			}
		}
		ev, conf := evidence.event, events[0].event.Confidence
		return &verdictCandidate{reason: export.ReasonCompleted, state: StateYes, event: &ev, confidence: &conf, completedAt: &completedAt}
	}
	if len(manual) == 0 {
		return nil
	}
	latest := manual[0]
	for _, m := range manual[1:] {
		if latest.CompletedAt.Before(m.CompletedAt) {
			latest = m
		}
	}
	at := latest.CompletedAt
	return &verdictCandidate{
		reason: export.ReasonCompleted, state: StateYes,
		event: latest.RepresentativeEvent(), completedAt: &at,
	}
}

func (s *Snapshot) upcomingCandidate(t ResetType, now time.Time) *scheduleMatch {
	if tl := s.ResetTimeline; tl != nil && t == ResetGlobal {
		if tl.NextSchedule == nil {
			return nil
		}
		primary := *tl.NextSchedule
		if primary.ResetType.Includes(t) {
			w := s.ScheduledResetWindow(primary)
			if w == nil || !w.PendingUntil().After(now) {
				return nil
			}
			return &scheduleMatch{primary, *w}
		}
	}
	suppressed := s.suppressed()
	var best *scheduleMatch
	for _, e := range s.mergedScheduleCandidates() {
		if e.Kind != KindResetScheduled || !e.ResetType.Includes(t) || suppressed[e.Source.PostID] {
			continue
		}
		w := s.ScheduledResetWindow(e)
		if w == nil || !w.PendingUntil().After(now) {
			continue
		}
		if best == nil || w.StartAt.Before(best.window.StartAt) {
			best = &scheduleMatch{e, *w}
		}
	}
	return best
}

func (s *Snapshot) graceCandidate(t ResetType, now time.Time) *scheduleMatch {
	suppressed := s.suppressed()
	fulfilled := s.fulfilledSchedulePostIDs()
	var best *scheduleMatch
	for _, e := range s.mergedScheduleCandidates() {
		if e.Kind != KindResetScheduled || !e.ResetType.Includes(t) ||
			suppressed[e.Source.PostID] || fulfilled[e.Source.PostID] {
			continue
		}
		w := s.ScheduledResetWindow(e)
		if w == nil || w.PendingUntil().After(now) {
			continue
		}
		deadline, ok := e.graceDeadline(w.PendingUntil())
		if !ok || !w.PendingUntil().Before(deadline) || !now.Before(deadline) || s.hasCompletion(e, *w, now) {
			continue
		}
		cand := &scheduleMatch{e, *w}
		if best == nil || graceLess(*best, *cand) {
			best = cand
		}
	}
	return best
}

// graceLess is the max(by:) order: later pending end, then later announcement,
// then larger post ID.
func graceLess(a, b scheduleMatch) bool {
	if !a.window.PendingUntil().Equal(b.window.PendingUntil()) {
		return a.window.PendingUntil().Before(b.window.PendingUntil())
	}
	if !a.event.AnnouncedAt.Equal(b.event.AnnouncedAt) {
		return a.event.AnnouncedAt.Before(b.event.AnnouncedAt)
	}
	return a.event.Source.PostID < b.event.Source.PostID
}

func (s *Snapshot) suppressed() map[string]bool {
	if s.ResetTimeline == nil {
		return nil
	}
	return stringSet(s.ResetTimeline.SuppressedPostIDs)
}

func (s *Snapshot) fulfilledSchedulePostIDs() map[string]bool {
	ids := map[string]bool{}
	if s.ResetTimeline == nil {
		return ids
	}
	for _, f := range s.ResetTimeline.FulfilledSchedules {
		ids[f.Schedule.Source.PostID] = true
	}
	for _, m := range s.ResetTimeline.ManualCompletions {
		for _, id := range m.SchedulePostIDs {
			ids[id] = true
		}
		for _, e := range m.Schedules {
			ids[e.Source.PostID] = true
		}
	}
	return ids
}

func isWindow(w ScheduleWindow, day time.Time, loc *time.Location) bool {
	if !w.StartAt.After(day) && day.Before(w.PendingUntil()) {
		return true
	}
	if sameDay(w.StartAt, day, loc) {
		return true
	}
	return w.IsRange && sameDay(w.EndAt, day, loc)
}
