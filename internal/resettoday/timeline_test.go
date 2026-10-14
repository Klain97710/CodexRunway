package resettoday

import (
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
)

func manualJSON(id, completedAt, visibleUntil, rep string, schedules ...eventFixture) string {
	postIDs := ""
	for i, e := range schedules {
		if i > 0 {
			postIDs += ","
		}
		postIDs += `"` + e.PostID + `"`
	}
	return `{
      "id": "` + id + `",
      "completedAt": "` + completedAt + `",
      "visibleUntil": "` + visibleUntil + `",
      "representativePostId": "` + rep + `",
      "schedulePostIds": [` + postIDs + `],
      "schedules": [` + eventsJSON(schedules...) + `],
      "fulfillmentOrigin": "manual"
    }`
}

func TestSameDayManualCompletionIsYes(t *testing.T) {
	now := at(t, "2026-08-13T10:29:13Z")
	scheduled := newEvent("reset_scheduled", "2026-08-13T01:01:37Z", effective("2026-08-13T02:01:37Z"),
		precision("datetime"), basis("explicit"), post("2087706104814023111"))
	manual := manualJSON("manual:cl_abd57edf90eb450aa54bd48c010854af", "2026-08-13T04:35:00.000Z",
		"2026-08-23T04:35:00.000Z", "2087706104814023111", scheduled)
	s := newFeed(now, newEvent("reset_completed", "2026-08-11T00:27:44Z")).checked("2026-08-13T10:29:13Z").
		withTimeline(timelineJSON("", "", "", manual, `"2087706104814023111"`)).mustDecode(t)
	last := s.LatestResetAt(now)
	if !s.HasResetTimeline() || s.ResolvedState(now, time.UTC) != StateYes || !s.HasAlreadyEffectiveResetToday(now, time.UTC) ||
		s.NextScheduledReset(now) != nil || last == nil || !last.Equal(at(t, "2026-08-13T04:35:00Z")) ||
		s.PrimaryEvidenceEvent(now, time.UTC).Source.PostID != "2087706104814023111" {
		t.Fatal("manual completion state")
	}
	if got := s.EvidenceLine(english, now, time.UTC); got != "Confirmed without an X completion post." {
		t.Errorf("en = %q", got)
	}
	if got := s.EvidenceLine(chinese, now, time.UTC); got != "已确认发生重置，但没有对应的 X 完成帖。" {
		t.Errorf("zh = %q", got)
	}
}

func TestTimelineNextScheduleIsAuthoritative(t *testing.T) {
	now := at(t, "2026-08-01T12:00:00Z")
	leftover := newEvent("reset_scheduled", "2026-08-01T08:00:00Z", effective("2026-08-02T12:00:00Z"), post("2086189414292865249"))
	s := newFeed(now, leftover).checked("2026-08-01T12:00:00Z").
		withTimeline(timelineJSON("", "", "", "", `"2086189414292865249"`)).mustDecode(t)
	if !s.HasResetTimeline() || s.ResolvedState(now, time.UTC) != StateNo || s.NextScheduledReset(now) != nil ||
		s.PrimaryEvidenceEvent(now, time.UTC) != nil || s.EvidenceURL(now, time.UTC, l10n.English) != "" {
		t.Fatal("leftover schedule leaked past the timeline")
	}
}

func TestTimelineAwareCompletionWinsOverSameDaySchedule(t *testing.T) {
	now := at(t, "2026-08-01T12:00:00Z")
	completed := newEvent("reset_completed", "2026-08-01T11:50:00Z", post("100"))
	scheduled := newEvent("reset_scheduled", "2026-08-01T08:00:00Z", effective("2026-08-01T14:30:00Z"),
		precision("datetime"), post("200"))
	s := newFeed(now, completed, scheduled).checked("2026-08-01T12:00:00Z").
		withTimeline(timelineJSON(scheduled.JSON(), "200", "", "", "")).mustDecode(t)
	if s.ResolvedState(now, time.UTC) != StateYes || !s.HasAlreadyEffectiveResetToday(now, time.UTC) ||
		s.PrefersSameDayScheduleExplanation(now, time.UTC) ||
		s.PrimaryEvidenceEvent(now, time.UTC).Source.PostID != "100" ||
		s.NextScheduledReset(now).Event.Source.PostID != "200" {
		t.Fatal("completion does not win")
	}
}

func timelineSchedule(t *testing.T, now time.Time, check string, e eventFixture) *Snapshot {
	t.Helper()
	return newFeed(now, e).checked(check).withTimeline(timelineJSON(e.JSON(), e.PostID, "", "", "")).mustDecode(t)
}

func TestExplicitDatetimeAtMidnightStaysAPoint(t *testing.T) {
	now := at(t, "2026-08-10T06:59:59Z")
	e := newEvent("reset_scheduled", "2026-08-09T12:00:00Z", effective("2026-08-10T07:00:00Z"),
		precision("datetime"), post("2087706104814023111"))
	s := timelineSchedule(t, now, "2026-08-10T06:59:59Z", e)
	next := s.NextScheduledReset(now)
	mid := at(t, "2026-08-10T07:00:00Z")
	if next == nil || next.IsRange || !next.EffectiveAt.Equal(mid) || !next.EffectiveUntil.Equal(next.EffectiveAt) {
		t.Fatalf("next = %+v", next)
	}
	if s.NextScheduledReset(mid) != nil {
		t.Error("schedule still pending at its instant")
	}
}

func TestDatePrecisionNormalizesToTiboDay(t *testing.T) {
	now := at(t, "2026-08-10T12:00:00Z")
	e := newEvent("reset_scheduled", "2026-08-09T12:00:00Z", effective("2026-08-10T15:00:00Z"), precision("date"), post("200"))
	s := timelineSchedule(t, now, "2026-08-10T12:00:00Z", e)
	next := s.NextScheduledReset(now)
	if next == nil || !next.IsRange || !next.EffectiveAt.Equal(at(t, "2026-08-10T07:00:00Z")) ||
		!next.EffectiveUntil.Equal(at(t, "2026-08-11T06:59:00Z")) {
		t.Fatalf("next = %+v", next)
	}
	if w := s.ScheduledResetWindow(next.Event); w == nil || !w.PendingUntil().Equal(at(t, "2026-08-11T07:00:00Z")) {
		t.Errorf("window = %+v", w)
	}
}

func TestDateOnlyWindowStaysPendingUntilNextMidnight(t *testing.T) {
	now := at(t, "2026-08-11T06:59:30Z")
	e := newEvent("reset_scheduled", "2026-08-09T12:00:00Z", effective("2026-08-10T07:00:00Z"), precision("date"), post("200"))
	s := timelineSchedule(t, now, "2026-08-11T06:59:30Z", e)
	if n := s.NextScheduledReset(now); n == nil || n.Event.Source.PostID != "200" {
		t.Fatalf("next = %+v", n)
	}
	if s.NextScheduledReset(at(t, "2026-08-11T07:00:00Z")) != nil {
		t.Error("still pending after the window")
	}
}

func TestContextualInferenceScheduleUsesPreviewCopy(t *testing.T) {
	now := at(t, "2026-08-10T20:00:00Z")
	la := mustLoc(t, "America/Los_Angeles")
	s := newFeed(now, newEvent("reset_scheduled", "2026-08-09T12:00:00Z", effective("2026-08-10T07:00:00Z"),
		precision("date"), basis("contextual_inference"))).checked("2026-08-10T20:00:00Z").using(la).mustDecode(t)
	if s.ResolvedState(now, la) != StateYes || s.Events[0].ScheduleBasis != BasisContextualInference {
		t.Fatal("contextual schedule")
	}
	if got := s.EvidenceLine(english, now, la); got != "A high-probability Codex quota reset was inferred from context." {
		t.Errorf("en = %q", got)
	}
	if got := s.EvidenceLine(chinese, now, la); got != "根据上下文推断出高概率的 Codex 配额重置预告。" {
		t.Errorf("zh = %q", got)
	}
}

func TestManualCompletionFiresDetectedAlert(t *testing.T) {
	now := at(t, "2026-08-13T10:29:13Z")
	previous := newFeed(now).checked("2026-08-13T10:29:13Z").withTimeline(emptyTimeline()).mustDecode(t)
	scheduled := newEvent("reset_scheduled", "2026-08-13T01:01:37Z", effective("2026-08-13T02:01:37Z"),
		precision("datetime"), post("2087706104814023111"))
	current := newFeed(now).checked("2026-08-13T10:29:13Z").withTimeline(timelineJSON("", "", "", manualJSON(
		"manual:cl_test", "2026-08-13T04:35:00.000Z", "2026-08-23T04:35:00.000Z", "2087706104814023111", scheduled), "")).mustDecode(t)
	got := Alerts(previous, current, now, time.UTC)
	if len(got) != 1 || got[0].Kind != "rateLimitResetDetected" || got[0].Name != "2087706104814023111" ||
		!got[0].Date.Equal(at(t, "2026-08-13T04:35:00Z")) {
		t.Fatalf("alerts = %+v", got)
	}
}

const extraFieldsFeed = `{
  "schemaVersion": 1,
  "generatedAt": "2026-08-13T10:29:13.809Z",
  "lastSuccessfulCheckAt": "2026-08-13T10:29:13.809Z",
  "monitor": {"status": "ok", "errorCode": null},
  "events": [{
    "kind": "reset_completed", "resetType": "global",
    "announcedAt": "2026-08-11T00:27:44.000Z", "effectiveAt": null,
    "scope": {"plans": ["all"], "windows": ["unknown"]},
    "source": {"handle": "thsottiaux", "postId": "2086972802457063486", "url": "https://x.com/thsottiaux/status/2086972802457063486"},
    "confidence": 0.97, "rationale": "Explicit Codex quota reset announcement.", "text": "Hi.\n\nIt is done."
  }],
  "heatmap": {"timezone": "UTC", "weeks": 53, "total": 1, "days": [{"date": "2026-08-13", "count": 1, "level": 1,
    "sources": [{"postId": "2087706104814023111", "kind": "reset_completed", "resetType": "banked"}]}]},
  "resetTimeline": {
    "nextSchedule": null, "recentNonCompletedPostId": null,
    "fulfilledSchedules": [{
      "schedule": {"kind": "reset_scheduled", "resetType": "global", "announcedAt": "2026-08-08T20:34:50.000Z",
        "effectiveAt": "2026-08-10T07:00:00.000Z", "schedulePrecision": "date",
        "scope": {"plans": ["all"], "windows": ["weekly"]},
        "source": {"handle": "thsottiaux", "postId": "2086189414292865249", "url": "https://x.com/thsottiaux/status/2086189414292865249"},
        "confidence": 0.92, "rationale": "Explicit Codex quota reset schedule.", "text": "I'll do another performative reset on Monday"},
      "window": {"startAt": "2026-08-10T07:00:00.000Z", "endAt": "2026-08-11T07:00:00.000Z", "precision": "date"},
      "completionPostId": "2086972802457063486", "completedAt": "2026-08-11T00:27:44.000Z", "visibleUntil": "2026-08-21T00:27:44.000Z"}],
    "fulfilledBanked": [{
      "banked": {"kind": "banked_reset", "resetType": "banked", "announcedAt": "2026-08-11T00:20:00.000Z", "effectiveAt": null,
        "scope": {"plans": ["all"], "windows": ["unknown"]},
        "source": {"handle": "thsottiaux", "postId": "2086972802457063400", "url": "https://x.com/thsottiaux/status/2086972802457063400"},
        "confidence": 0.97, "rationale": "Banked reset announcement; not a completed reset.", "text": "A reset-bank credit will be available."},
      "completionPostId": "2086972802457063486", "completedAt": "2026-08-11T00:27:44.000Z", "visibleUntil": "2026-08-21T00:27:44.000Z"}],
    "manualCompletions": [{
      "id": "manual:cl_abd57edf90eb450aa54bd48c010854af", "completedAt": "2026-08-13T04:35:00.000Z",
      "visibleUntil": "2026-08-23T04:35:00.000Z", "representativePostId": "2087706104814023111",
      "schedulePostIds": ["2087706104814023111"],
      "schedules": [{"kind": "reset_scheduled", "resetType": "global", "announcedAt": "2026-08-13T01:01:37.000Z",
        "effectiveAt": "2026-08-13T02:01:37.000Z", "schedulePrecision": "datetime", "scheduleBasis": "explicit",
        "scope": {"plans": ["all"], "windows": ["unknown"]},
        "source": {"handle": "thsottiaux", "postId": "2087706104814023111", "url": "https://x.com/thsottiaux/status/2087706104814023111"},
        "confidence": 0.97, "rationale": "Explicit Codex quota reset schedule.", "text": "Enjoy a nice reset everyone. Landing in the next hour or so."}],
      "fulfillmentOrigin": "manual"}],
    "suppressedPostIds": ["2086189414292865249", "2087706104814023111"]}
}`

func TestExtraOfficialFieldsDoNotBreakDecoding(t *testing.T) {
	now := at(t, "2026-08-13T10:29:13Z")
	cn := mustLoc(t, "Asia/Shanghai")
	s, err := Decode([]byte(extraFieldsFeed), now, cn)
	if err != nil {
		t.Fatal(err)
	}
	tl := s.ResetTimeline
	last := s.LatestResetAt(now)
	if !s.HasResetTimeline() || len(tl.FulfilledSchedules) != 1 || tl.FulfilledSchedules[0].Schedule.ResetType != ResetGlobal ||
		len(tl.FulfilledBanked) != 1 || tl.FulfilledBanked[0].Banked.Kind != KindBankedReset ||
		tl.FulfilledBanked[0].Banked.ResetType != ResetBanked ||
		len(tl.ManualCompletions) != 1 || tl.ManualCompletions[0].Schedules[0].ResetType != ResetGlobal ||
		len(tl.SuppressedPostIDs) != 2 || s.ResolvedState(now, cn) != StateYes ||
		last == nil || !last.Equal(at(t, "2026-08-13T04:35:00Z")) {
		t.Fatalf("timeline = %+v", tl)
	}
	if s.Events[0].Text != "Hi.\n\nIt is done." {
		t.Errorf("text = %q", s.Events[0].Text)
	}
}

func TestRejectsMismatchedContextualRationale(t *testing.T) {
	now := at(t, "2026-08-10T12:00:00Z")
	f := newFeed(now)
	f.EventsJSON = `{
      "kind": "reset_scheduled", "announcedAt": "2026-08-09T12:00:00Z", "effectiveAt": "2026-08-10T15:00:00Z",
      "schedulePrecision": "datetime", "scope": {"plans": ["all"], "windows": ["weekly"]},
      "source": {"handle": "thsottiaux", "postId": "123", "url": "https://x.com/thsottiaux/status/123"},
      "confidence": 0.9, "rationale": "High-probability Codex quota reset preview inferred from context.", "text": "Maybe tomorrow."}`
	wantDecodeError(t, f)
}
