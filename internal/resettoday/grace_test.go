package resettoday

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/export"
)

func graceFixture(hoursJSON string) eventFixture {
	opts := []eventOption{effective("2026-09-25T07:00:00Z"), precision("date"), basis("explicit")}
	if hoursJSON != "" {
		opts = append(opts, graceHours(hoursJSON))
	}
	return newEvent("reset_scheduled", "2026-09-24T12:00:00Z", opts...)
}

func graceEvent(when time.Time, hours float64, t ResetType, postID string) Event {
	e := scheduledEvent(when, 0.9, postID, t)
	e.ScheduleGraceHours = &hours
	if t == ResetBanked {
		e.Rationale = "Explicit Codex reset-bank credit schedule."
	}
	return e
}

func TestDecodesGraceHours(t *testing.T) {
	now := at(t, "2026-09-26T12:00:00Z")
	missing := newFeed(now, graceFixture("")).mustDecode(t)
	if missing.Events[0].ScheduleGraceHours != nil || missing.Events[0].graceInterval() != 3*3600 || len(missing.GraceSchedules) != 0 {
		t.Fatalf("missing hours = %+v", missing.Events[0])
	}
	for _, hours := range []string{"0", "1.5", "24", "72", "100"} {
		e := graceFixture(hours)
		s := newFeed(now, e).withGrace(e).mustDecode(t)
		var want float64
		fmt.Sscan(hours, &want)
		if g := s.Events[0].ScheduleGraceHours; g == nil || *g != want {
			t.Errorf("%s: hours = %v", hours, g)
		}
		if !reflect.DeepEqual(s.GraceSchedules, s.Events) {
			t.Errorf("%s: graceSchedules differ from events", hours)
		}
	}
}

func TestRejectsInvalidGracePayloads(t *testing.T) {
	now := at(t, "2026-09-26T12:00:00Z")
	for _, raw := range []string{"null", "true", `"3"`, "-1", "1e308"} {
		t.Run(raw, func(t *testing.T) { wantDecodeError(t, newFeed(now, graceFixture(raw))) })
	}
	_, err := Decode([]byte(`{"schemaVersion":1,"generatedAt":"2026-09-26T12:00:00Z","lastSuccessfulCheckAt":"2026-09-26T12:00:00Z","monitor":{"status":"ok","errorCode":null},"events":[],"graceSchedules":null}`), now, nil)
	if err == nil {
		t.Error("null graceSchedules decoded")
	}
}

func TestValidatesGraceScheduleContract(t *testing.T) {
	now := at(t, "2026-09-26T12:00:00Z")
	scheduled := graceFixture("24")
	completed := newEvent("reset_completed", "2026-09-25T12:00:00Z")
	tests := []struct {
		name string
		feed feedFixture
	}{
		{"completed kind", newFeed(now).withGrace(completed)},
		{"missing duration", newFeed(now).withGrace(graceFixture(""))},
		{"duplicates", newFeed(now).withGrace(scheduled, scheduled)},
		{"identity differs from event", newFeed(now, graceFixture("3")).withGrace(scheduled)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { wantDecodeError(t, tc.feed) })
	}
}

func TestGraceOnlyCandidateUsesItsOwnDeadline(t *testing.T) {
	for _, hours := range []float64{24, 72, 100} {
		t.Run(fmt.Sprint(hours), func(t *testing.T) {
			due := at(t, "2026-09-25T07:00:00Z")
			e := graceEvent(due, hours, ResetGlobal, "1")
			deadline := due.Add(time.Duration(hours) * time.Hour)
			inside := deadline.Add(-time.Second)
			s := makeSnapshot(inside, snapOpts{graceSchedules: []Event{e}})
			if len(s.Events) != 0 {
				t.Fatal("events not empty")
			}
			if got := s.VerdictPresentation(inside, time.UTC).Reason; got != export.ReasonGrace {
				t.Fatalf("inside: %v", got)
			}
			p := s.VerdictPresentation(deadline, time.UTC)
			if p.Reason == export.ReasonGrace || p.ShowsYes || p.IsCompleted {
				t.Errorf("at deadline: %+v", p)
			}
			if got, ok := e.graceDeadline(due); !ok || !got.Equal(deadline) {
				t.Errorf("deadline = %v/%v", got, ok)
			}
			found := false
			for _, d := range s.TransitionDates(inside.Add(-time.Second), time.UTC) {
				found = found || d.Equal(deadline)
			}
			if !found {
				t.Error("deadline is not a transition date")
			}
		})
	}
}

func TestGraceProjectionIsIndependentFromEvents(t *testing.T) {
	due := at(t, "2026-09-25T12:00:00Z")
	now := due.Add(time.Hour)
	var history []Event
	for offset := 1; offset <= 6; offset++ {
		e := completedEvent(due.Add(-time.Duration(offset)*24*time.Hour), 0.98, fmt.Sprint(offset+10))
		history = append(history, e)
	}
	s := makeSnapshot(now, snapOpts{events: history, graceSchedules: []Event{graceEvent(due, 24, ResetGlobal, "1")}})
	if len(s.Events) != 6 || s.VerdictPresentation(now, time.UTC).Reason != export.ReasonGrace {
		t.Fatalf("reason = %v", s.VerdictPresentation(now, time.UTC).Reason)
	}
}

func TestZeroAndDecimalGrace(t *testing.T) {
	due := at(t, "2026-09-25T12:00:00Z")
	disabled := makeSnapshot(due, snapOpts{graceSchedules: []Event{graceEvent(due, 0, ResetGlobal, "1")}})
	if got := disabled.VerdictPresentation(due, time.UTC).Reason; got != export.ReasonExpiredUnconfirmed {
		t.Errorf("zero hours: %v", got)
	}
	decimal := makeSnapshot(due, snapOpts{graceSchedules: []Event{graceEvent(due, 1.5, ResetGlobal, "1")}})
	if got := decimal.VerdictPresentation(due.Add(5399*time.Second), time.UTC).Reason; got != export.ReasonGrace {
		t.Errorf("5399s: %v", got)
	}
	if got := decimal.VerdictPresentation(due.Add(5400*time.Second), time.UTC).Reason; got != export.ReasonExpiredUnconfirmed {
		t.Errorf("5400s: %v", got)
	}
}

func TestResetTypesUseIndependentDurations(t *testing.T) {
	due := at(t, "2026-09-25T12:00:00Z")
	s := makeSnapshot(due, snapOpts{graceSchedules: []Event{
		graceEvent(due, 1, ResetGlobal, "1"), graceEvent(due, 4, ResetBanked, "2"),
	}})
	p := s.VerdictPresentation(due.Add(2*time.Hour), time.UTC)
	if p.Reason != export.ReasonGrace || p.ResetType == nil || *p.ResetType != ResetBanked || p.EvidenceEvent.Source.PostID != "2" {
		t.Fatalf("presentation = %+v", p)
	}
}

func TestDateCompletionPreventsCrossDayGrace(t *testing.T) {
	eff := at(t, "2026-11-01T15:00:00Z")
	schedule := graceEvent(eff, 24, ResetGlobal, "1")
	schedule.SchedulePrecision = PrecisionDate
	schedule.AnnouncedAt = at(t, "2026-11-01T06:00:00Z")
	completion := completedEvent(at(t, "2026-11-01T20:00:00Z"), 0.98, "2")
	now := at(t, "2026-11-02T09:00:00Z")
	s := makeSnapshot(now, snapOpts{events: []Event{completion}, graceSchedules: []Event{schedule}})
	if w := s.ScheduledResetWindow(schedule); w == nil || !w.StartAt.Equal(at(t, "2026-11-01T07:00:00Z")) {
		t.Fatalf("window = %+v", w)
	}
	if got := s.VerdictPresentation(now, time.UTC).Reason; got != export.ReasonExpiredUnconfirmed {
		t.Errorf("reason = %v", got)
	}
}

func TestCompletionMatchingUsesFullContract(t *testing.T) {
	due := at(t, "2026-09-25T20:00:00Z")
	schedule := graceEvent(due, 24, ResetGlobal, "1")
	schedule.AnnouncedAt = at(t, "2026-09-25T15:00:00Z")
	schedule.Scope = Scope{Plans: []string{"plus"}, Windows: []string{"weekly"}}
	now := due.Add(5 * time.Hour)

	tooEarly := completedEvent(at(t, "2026-09-25T06:59:59Z"), 0.98, "2")
	tooEarly.AnnouncedAt = at(t, "2026-09-25T16:00:00Z")
	tooEarly.Scope = schedule.Scope
	matching := tooEarly
	eff := at(t, "2026-09-25T07:00:00Z")
	matching.EffectiveAt = &eff
	announcedTooSoon := matching
	announcedTooSoon.AnnouncedAt = schedule.AnnouncedAt.Add(-time.Second)
	wrongType := matching
	wrongType.ResetType = ResetBanked
	wrongScope := matching
	wrongScope.Scope = Scope{Plans: []string{"enterprise"}, Windows: []string{"weekly"}}

	tests := []struct {
		name  string
		event Event
		want  export.VerdictReason
	}{
		{"before the lower bound", tooEarly, export.ReasonGrace},
		{"matching", matching, export.ReasonNone},
		{"announced before the schedule", announcedTooSoon, export.ReasonGrace},
		{"wrong type", wrongType, export.ReasonGrace},
		{"wrong scope", wrongScope, export.ReasonGrace},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := makeSnapshot(now, snapOpts{events: []Event{tc.event}, graceSchedules: []Event{schedule}})
			if got := s.VerdictPresentation(now, time.UTC).Reason; got != tc.want {
				t.Fatalf("reason = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestManualCompletionUsesWildcardScope(t *testing.T) {
	due := at(t, "2026-09-25T20:00:00Z")
	schedule := graceEvent(due, 24, ResetGlobal, "1")
	schedule.Scope = Scope{Plans: []string{"plus"}, Windows: []string{"weekly"}}
	reference := graceEvent(due, 3, ResetGlobal, "9")
	reference.Scope = Scope{Plans: []string{"enterprise"}, Windows: []string{"five_hour"}}
	manual := ManualCompletion{
		ID: "manual:1", CompletedAt: due.Add(time.Minute), VisibleUntil: due.Add(10 * time.Hour),
		RepresentativePostID: "9", SchedulePostIDs: []string{"9"}, Schedules: []Event{reference},
	}
	now := due.Add(5 * time.Hour)
	s := makeSnapshot(now, snapOpts{graceSchedules: []Event{schedule}, timeline: &Timeline{ManualCompletions: []ManualCompletion{manual}}})
	if got := s.VerdictPresentation(now, time.UTC).Reason; got != export.ReasonNone {
		t.Fatalf("reason = %v", got)
	}
}
