package resettoday

import (
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/export"
)

func pendingScheduleSnapshot(t *testing.T, confidence float64, now time.Time) *Snapshot {
	t.Helper()
	s := newFeed(now, newEvent("reset_scheduled", "2026-07-28T09:00:00Z",
		effective("2026-07-28T13:00:00Z"), precision("datetime"))).mustDecode(t)
	s.Events[0].Confidence = confidence
	return s
}

func TestCompletedResetOmitsPercent(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	s := newFeed(now, newEvent("reset_completed", "2026-07-28T04:09:02Z", effective("2026-07-28T04:09:02Z"))).mustDecode(t)
	p := s.VerdictPresentation(now, time.UTC)
	if !p.ShowsYes || !p.IsCompleted || p.IsScheduled || p.Percent != nil || p.Band != nil {
		t.Fatalf("presentation = %+v", p)
	}
	if p.ResetType == nil || *p.ResetType != ResetGlobal {
		t.Fatalf("reset type = %v", p.ResetType)
	}
	if got := p.TitleText(english); got != "Yes" {
		t.Errorf("en title = %q", got)
	}
	if got := p.TitleText(chinese); got != "是" {
		t.Errorf("zh title = %q", got)
	}
}

func TestScheduledResetShowsPercentAndBand(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	s := pendingScheduleSnapshot(t, 0.98, now)
	tests := []struct {
		confidence float64
		percent    int
		band       export.ConfidenceBand
		enPercent  string
		enTitle    string
		zhTitle    string
	}{
		{0.98, 98, export.BandOK, "≥98%", "≥98%Yes", "≥98%是"},
		{0.6, 60, export.BandWarn, "≥60%", "≥60%Yes", "≥60%是"},
		{1, 100, export.BandOK, "100%", "100%Yes", "100%是"},
	}
	for _, tc := range tests {
		s.Events[0].Confidence = tc.confidence
		p := s.VerdictPresentation(now, time.UTC)
		if !p.ShowsYes || !p.IsScheduled || p.IsCompleted {
			t.Fatalf("%v: presentation = %+v", tc.confidence, p)
		}
		if p.Percent == nil || *p.Percent != tc.percent || p.Band == nil || *p.Band != tc.band {
			t.Errorf("%v: percent/band = %v/%v", tc.confidence, p.Percent, p.Band)
		}
		if got := p.PercentText(english); got != tc.enPercent {
			t.Errorf("%v: percent text = %q", tc.confidence, got)
		}
		if got := p.TitleText(english); got != tc.enTitle {
			t.Errorf("%v: en title = %q", tc.confidence, got)
		}
		if got := p.TitleText(chinese); got != tc.zhTitle {
			t.Errorf("%v: zh title = %q", tc.confidence, got)
		}
	}
}

func TestCompletedResetOutranksSchedule(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	s := newFeed(now,
		newEvent("reset_completed", "2026-07-28T04:09:02Z", effective("2026-07-28T04:09:02Z"), post("1")),
		newEvent("reset_scheduled", "2026-07-28T11:00:00Z", effective("2026-07-29T13:00:00Z"),
			precision("datetime"), post("2")),
	).mustDecode(t)
	s.Events[1].Confidence = 0.6
	p := s.VerdictPresentation(now, time.UTC)
	if !p.IsCompleted || p.IsScheduled || p.Percent != nil {
		t.Fatalf("presentation = %+v", p)
	}
	if got := ScheduleConfidenceBand(s.Events[1]); got != export.BandWarn {
		t.Errorf("band = %v", got)
	}
}

func TestGraceUsesHalfOpenThreeHourWindow(t *testing.T) {
	pendingUntil := at(t, "2026-07-28T13:00:00Z")
	s := makeSnapshot(pendingUntil, snapOpts{events: []Event{scheduledEvent(pendingUntil, 0.92, "1", ResetGlobal)}})
	tests := []struct {
		offset time.Duration
		reason export.VerdictReason
	}{
		{0, export.ReasonGrace},
		{3*time.Hour - time.Second, export.ReasonGrace},
		{3 * time.Hour, export.ReasonExpiredUnconfirmed},
	}
	for _, tc := range tests {
		p := s.VerdictPresentation(pendingUntil.Add(tc.offset), time.UTC)
		if p.Reason != tc.reason {
			t.Fatalf("+%v: reason = %v, want %v", tc.offset, p.Reason, tc.reason)
		}
		if tc.reason == export.ReasonGrace && (!p.ShowsYes || p.Percent == nil || *p.Percent != 92) {
			t.Errorf("+%v: grace presentation = %+v", tc.offset, p)
		}
		if tc.reason == export.ReasonExpiredUnconfirmed &&
			(p.ShowsYes || p.Confidence != nil || p.ScheduleWindow != nil || p.EvidenceEvent != nil) {
			t.Errorf("+%v: expired keeps evidence: %+v", tc.offset, p)
		}
	}
}

func TestFreshnessUsesNewestProducerTimestamp(t *testing.T) {
	generated := at(t, "2026-07-28T12:00:00Z")
	stale := generated.Add(-40 * time.Hour)
	s := makeSnapshot(generated, snapOpts{generatedAt: &generated, lastSuccessfulCheckAt: &stale})
	if !s.FreshnessAt().Equal(generated) {
		t.Fatalf("freshnessAt = %v", s.FreshnessAt())
	}
	if got := s.VerdictPresentation(generated.Add(30*time.Hour), time.UTC).Reason; got != export.ReasonNone {
		t.Errorf("at 30h: %v", got)
	}
	if got := s.VerdictPresentation(generated.Add(30*time.Hour+time.Second), time.UTC).Reason; got != export.ReasonUnavailable {
		t.Errorf("at 30h+1s: %v", got)
	}
}

func TestMatchingSchedulesMergeTypesAndConfidence(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	when := at(t, "2026-07-29T12:00:00Z")
	global := scheduledEvent(when, 0.92, "1", ResetGlobal)
	banked := scheduledEvent(when, 0.76, "2", ResetBanked)
	banked.ScheduleBasis = BasisContextualInference
	p := makeSnapshot(now, snapOpts{events: []Event{global, banked}}).VerdictPresentation(now, time.UTC)
	if p.Reason != export.ReasonUpcoming || p.ResetType == nil || *p.ResetType != ResetGlobalAndBanked {
		t.Fatalf("presentation = %+v", p)
	}
	if p.Confidence == nil || *p.Confidence != 0.76 || *p.Percent != 76 || *p.Band != export.BandWarn {
		t.Errorf("confidence/percent/band = %v/%v/%v", p.Confidence, p.Percent, p.Band)
	}
	if p.ScheduleBasis != BasisExplicit {
		t.Errorf("basis = %q", p.ScheduleBasis)
	}
}

func TestContextualScheduleRemainsPrediction(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	e := scheduledEvent(at(t, "2026-07-28T13:00:00Z"), 0.81, "1", ResetGlobal)
	e.ScheduleBasis = BasisContextualInference
	s := makeSnapshot(now, snapOpts{events: []Event{e}})
	p := s.VerdictPresentation(now, time.UTC)
	if p.Reason != export.ReasonUpcoming || !p.ShowsYes || p.IsCompleted {
		t.Fatalf("presentation = %+v", p)
	}
	if s.ResolvedState(now, time.UTC) != StateYes {
		t.Error("state is not yes")
	}
}

func TestSuppressedAndFulfilledSchedulesCannotEnterGrace(t *testing.T) {
	now := at(t, "2026-07-28T13:30:00Z")
	when := at(t, "2026-07-28T13:00:00Z")
	event := scheduledEvent(when, 0.92, "9", ResetGlobal)
	fulfilled := FulfilledSchedule{
		Schedule:         event,
		Window:           PublishedWindow{StartAt: when, EndAt: when, Precision: PrecisionDatetime},
		CompletionPostID: "10",
		CompletedAt:      when,
		VisibleUntil:     now.Add(time.Hour),
	}
	tests := []struct {
		name     string
		timeline *Timeline
	}{
		{"suppressed", &Timeline{SuppressedPostIDs: []string{"9"}}},
		{"fulfilled", &Timeline{FulfilledSchedules: []FulfilledSchedule{fulfilled}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := makeSnapshot(now, snapOpts{events: []Event{event}, timeline: tc.timeline}).VerdictPresentation(now, time.UTC)
			if p.Reason != export.ReasonExpiredUnconfirmed {
				t.Fatalf("reason = %v", p.Reason)
			}
			if p.ResetType != nil || p.Confidence != nil || p.EvidenceEvent != nil {
				t.Errorf("expired keeps evidence: %+v", p)
			}
		})
	}
}

func TestManualCompletionIsCompleted(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	manual := ManualCompletion{
		ID: "manual:1", CompletedAt: now.Add(-300 * time.Second), VisibleUntil: now.Add(time.Hour),
		RepresentativePostID: "1",
	}
	p := makeSnapshot(now, snapOpts{timeline: &Timeline{ManualCompletions: []ManualCompletion{manual}}}).
		VerdictPresentation(now, time.UTC)
	if p.Reason != export.ReasonCompleted || p.ResetType == nil || *p.ResetType != ResetGlobal {
		t.Fatalf("presentation = %+v", p)
	}
	if p.CompletedAt == nil || !p.CompletedAt.Equal(manual.CompletedAt) || p.Percent != nil || p.Confidence != nil {
		t.Errorf("completedAt/percent/confidence = %v/%v/%v", p.CompletedAt, p.Percent, p.Confidence)
	}
}

func TestLosAngelesDateScheduleKeepsDSTBoundary(t *testing.T) {
	start := at(t, "2026-03-08T08:00:00Z")
	e := scheduledEvent(start, 0.9, "1", ResetGlobal)
	e.SchedulePrecision = PrecisionDate
	pendingUntil := at(t, "2026-03-09T07:00:00Z")
	s := makeSnapshot(pendingUntil, snapOpts{events: []Event{e}, generatedAt: &pendingUntil})
	w := s.ScheduledResetWindow(e)
	if w == nil || !w.PendingUntil().Equal(pendingUntil) {
		t.Fatalf("window = %+v", w)
	}
	if got := s.VerdictPresentation(pendingUntil, time.UTC).Reason; got != export.ReasonGrace {
		t.Errorf("reason = %v", got)
	}
}

func TestCompletionEndsAtLocalMidnight(t *testing.T) {
	now := at(t, "2026-07-29T00:00:00Z")
	e := completedEvent(now.Add(-time.Second), 0.98, "1")
	if got := makeSnapshot(now, snapOpts{events: []Event{e}}).VerdictPresentation(now, time.UTC).Reason; got != export.ReasonNone {
		t.Errorf("reason = %v", got)
	}
}

func TestTransitionDatesCoverEveryBoundary(t *testing.T) {
	now := at(t, "2026-07-28T23:00:00Z")
	scheduleAt := at(t, "2026-07-29T08:00:00Z")
	next := scheduledEvent(scheduleAt, 0.9, "1", ResetGlobal)
	manual := ManualCompletion{
		ID: "manual:1", CompletedAt: now.Add(600 * time.Second), VisibleUntil: now.Add(1200 * time.Second),
		RepresentativePostID: "1",
	}
	s := makeSnapshot(now, snapOpts{timeline: &Timeline{NextSchedule: &next, ManualCompletions: []ManualCompletion{manual}}})
	dates := s.TransitionDates(now, time.UTC)
	want := []time.Time{
		scheduleAt, scheduleAt.Add(3 * time.Hour), manual.CompletedAt, manual.VisibleUntil.Add(time.Second),
		now.Add(30*time.Hour + time.Second), at(t, "2026-07-29T00:00:00Z"), at(t, "2026-07-30T00:00:00Z"),
	}
	for _, w := range want {
		found := false
		for _, d := range dates {
			if d.Equal(w) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing transition %v in %v", w, dates)
		}
	}
	for _, d := range dates {
		if d.Nanosecond() != 0 {
			t.Errorf("fractional transition %v", d)
		}
	}
}

func TestCompletedStatusKeepsFeedPriorityWhileEvidenceUsesLatest(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	first := completedEvent(now.Add(-time.Hour), 0.6, "1")
	first.AnnouncedAt = now.Add(-3500 * time.Second)
	second := completedEvent(now.Add(-1800*time.Second), 0.98, "2")
	manual := ManualCompletion{
		ID: "manual:1", CompletedAt: now.Add(-600 * time.Second), VisibleUntil: now.Add(time.Hour),
		RepresentativePostID: "3",
	}
	s := makeSnapshot(now, snapOpts{events: []Event{first, second}, timeline: &Timeline{ManualCompletions: []ManualCompletion{manual}}})
	p := s.VerdictPresentation(now, time.UTC)
	if p.Confidence == nil || *p.Confidence != 0.6 {
		t.Errorf("confidence = %v", p.Confidence)
	}
	if p.EvidenceEvent == nil || p.EvidenceEvent.Source.PostID != "2" {
		t.Errorf("evidence = %+v", p.EvidenceEvent)
	}
	if p.CompletedAt == nil || !p.CompletedAt.Equal(manual.CompletedAt) {
		t.Errorf("completedAt = %v", p.CompletedAt)
	}
}

func TestDevMocksCoverLifecycle(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	tests := []struct {
		kind   DevMockKind
		reason export.VerdictReason
	}{
		{MockCompleted, export.ReasonCompleted},
		{MockExplicitScheduled, export.ReasonUpcoming},
		{MockInferredScheduled, export.ReasonUpcoming},
		{MockGrace, export.ReasonGrace},
		{MockExpired, export.ReasonExpiredUnconfirmed},
		{MockUnavailable, export.ReasonUnavailable},
	}
	for _, tc := range tests {
		if got := DevMock(tc.kind, now, time.UTC).VerdictPresentation(now, time.UTC).Reason; got != tc.reason {
			t.Errorf("%v: reason = %v, want %v", tc.kind, got, tc.reason)
		}
	}
}

func TestUnavailableDetailExplainsSafeCause(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	old := now.Add(-31 * time.Hour)
	stale := makeSnapshot(now, snapOpts{generatedAt: &old, lastSuccessfulCheckAt: &old})
	code := "secret_detail"
	degraded := makeSnapshot(now, snapOpts{monitor: &Monitor{Status: MonitorDegraded, ErrorCode: &code}})
	noFmt := FormatterFuncs{}
	if d := stale.VerdictDetail(english, noFmt, now, time.UTC); d == nil || d.PlainText == "" {
		t.Fatalf("stale detail = %+v", d)
	}
	sd := stale.VerdictDetail(english, noFmt, now, time.UTC).PlainText
	dd := degraded.VerdictDetail(english, noFmt, now, time.UTC).PlainText
	if sd == dd {
		t.Error("stale and monitor causes share text")
	}
	if contains(dd, "secret_detail") {
		t.Errorf("monitor detail leaks the error code: %q", dd)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
