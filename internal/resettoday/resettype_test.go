package resettoday

import (
	"strings"
	"testing"
	"time"
)

func TestDecodesResetTypes(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	tests := []struct {
		raw  string
		want ResetType
	}{
		{"", ResetGlobal}, {"global", ResetGlobal}, {"banked", ResetBanked}, {"global_and_banked", ResetGlobalAndBanked},
	}
	for _, tc := range tests {
		s := newFeed(now, newEvent("reset_completed", "2026-07-28T11:00:00Z", rtype(tc.raw))).mustDecode(t)
		if got := s.Events[0].ResetType; got != tc.want {
			t.Errorf("%q: %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestRejectsInvalidResetTypes(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	valid := newEvent("reset_completed", "2026-07-28T11:00:00Z", rtype("global")).JSON()
	f := newFeed(now)
	f.EventsJSON = strings.Replace(valid, `"resetType": "global"`, `"resetType": null`, 1)
	wantDecodeError(t, f)
	wantDecodeError(t, newFeed(now, newEvent("reset_completed", "2026-07-28T11:00:00Z", rtype("future_type"))))
}

func TestValidatesKindAndRationale(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	banked := newFeed(now, newEvent("banked_reset", "2026-07-28T11:00:00Z", rtype("banked"))).mustDecode(t)
	if banked.Events[0].ResetType != ResetBanked {
		t.Fatal("banked_reset type")
	}
	for _, e := range []eventFixture{
		newEvent("banked_reset", "2026-07-28T11:00:00Z", rtype("global_and_banked")),
		newEvent("limit_increase", "2026-07-28T11:00:00Z", rtype("banked")),
		newEvent("uncertain", "2026-07-28T11:00:00Z", rtype("banked")),
	} {
		wantDecodeError(t, newFeed(now, e))
	}
	f := newFeed(now)
	f.EventsJSON = strings.Replace(newEvent("reset_completed", "2026-07-28T11:00:00Z", rtype("banked")).JSON(),
		"Explicit Codex reset-bank credit announcement.", "Explicit Codex quota reset announcement.", 1)
	wantDecodeError(t, f)
}

func TestAcceptsCanonicalTypedRationales(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	for i, rt := range []string{"global", "banked", "global_and_banked"} {
		id := func(p string) eventOption { return post(p + string(rune('0'+i))) }
		s := newFeed(now,
			newEvent("reset_completed", "2026-07-28T11:00:00Z", rtype(rt), id("10")),
			newEvent("reset_scheduled", "2026-07-28T10:00:00Z", rtype(rt), effective("2026-07-28T14:00:00Z"),
				precision("datetime"), basis("explicit"), id("20")),
			newEvent("reset_scheduled", "2026-07-28T10:00:00Z", rtype(rt), effective("2026-07-28T15:00:00Z"),
				precision("datetime"), basis("contextual_inference"), id("30")),
		).mustDecode(t)
		if len(s.Events) != 3 {
			t.Errorf("%s: %d events", rt, len(s.Events))
		}
	}
}

func TestTimelineNextScheduleRequiresGlobal(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	banked := newEvent("reset_scheduled", "2026-07-28T10:00:00Z", rtype("banked"), effective("2026-07-28T14:00:00Z"), post("200"))
	wantDecodeError(t, newFeed(now, banked).withTimeline(timelineJSON(banked.JSON(), "200", "", "", "")))
}

func TestDisplayAndLatestResetTypes(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	s := newFeed(now,
		newEvent("reset_completed", "2026-07-28T10:00:00Z", rtype("global"), post("100")),
		newEvent("reset_completed", "2026-07-28T11:00:00Z", rtype("banked"), post("200")),
	).mustDecode(t)
	if got := s.DisplayResetType(now, time.UTC); got == nil || *got != ResetGlobalAndBanked {
		t.Errorf("display type = %v", got)
	}
	latest := s.LatestReset(now)
	if latest == nil || !latest.At.Equal(at(t, "2026-07-28T11:00:00Z")) || latest.ResetType != ResetBanked {
		t.Fatalf("latest = %+v", latest)
	}
	if at := s.LatestResetAt(now); at == nil || !at.Equal(latest.At) {
		t.Errorf("latestResetAt = %v", at)
	}
	if got := s.EvidenceLine(english, now, time.UTC); got != "A reset-bank credit was confirmed available for on-demand use." {
		t.Errorf("evidence = %q", got)
	}
}

func TestLatestResetMergesSameInstant(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	s := newFeed(now,
		newEvent("reset_completed", "2026-07-28T11:00:00Z", rtype("global"), post("100")),
		newEvent("reset_completed", "2026-07-28T11:00:00Z", rtype("banked"), post("200")),
	).mustDecode(t)
	if got := s.LatestReset(now); got == nil || got.ResetType != ResetGlobalAndBanked {
		t.Fatalf("latest = %+v", got)
	}
}

func TestTimelineIncludesBankedOnlySchedules(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	global := newEvent("reset_scheduled", "2026-07-28T10:00:00Z", rtype("global"), effective("2026-07-28T16:00:00Z"), post("100"))
	banked := newEvent("reset_scheduled", "2026-07-28T11:00:00Z", rtype("banked"), effective("2026-07-28T14:00:00Z"), post("200"))
	s := newFeed(now, global, banked).withTimeline(timelineJSON(global.JSON(), "100", "", "", "")).mustDecode(t)
	if s.State() != StateYes || s.NextScheduledReset(now).Event.Source.PostID != "200" {
		t.Fatal("banked-only schedule ignored")
	}
	if d := s.DisplayResetType(now, time.UTC); d == nil || *d != ResetBanked {
		t.Errorf("display = %v", d)
	}
	sup := newFeed(now, global, banked).withTimeline(timelineJSON(global.JSON(), "100", "", "", `"200"`)).mustDecode(t)
	if sup.NextScheduledReset(now).Event.Source.PostID != "100" {
		t.Error("suppressed banked schedule still wins")
	}
	if d := sup.DisplayResetType(now, time.UTC); d == nil || *d != ResetGlobal {
		t.Errorf("suppressed display = %v", d)
	}
}

func TestUpcomingSummaryMergesTypes(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	s := newFeed(now,
		newEvent("reset_scheduled", "2026-07-28T10:00:00Z", rtype("global"), effective("2026-07-28T16:00:00Z"), basis("explicit"), post("100")),
		newEvent("reset_scheduled", "2026-07-28T11:00:00Z", rtype("banked"), effective("2026-07-28T16:00:00Z"), basis("contextual_inference"), post("200")),
	).mustDecode(t)
	s.Events[0].Confidence = 0.92
	s.Events[1].Confidence = 0.76
	sum := s.NextScheduledResetSummary(now)
	if sum == nil || sum.ResetType != ResetGlobalAndBanked || sum.Confidence != 0.76 || sum.ScheduleBasis != BasisExplicit ||
		sum.Event.Source.PostID != "100" || sum.Event.ResetType != ResetGlobal {
		t.Fatalf("summary = %+v", sum)
	}
}

func TestTimelineIncludesMixedBankedSchedule(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	global := newEvent("reset_scheduled", "2026-07-28T10:00:00Z", rtype("global"), effective("2026-07-28T16:00:00Z"), post("100"))
	mixed := newEvent("reset_scheduled", "2026-07-28T11:00:00Z", rtype("global_and_banked"), effective("2026-07-28T14:00:00Z"), post("200"))
	s := newFeed(now, global, mixed).withTimeline(timelineJSON(global.JSON(), "", "", "", "")).mustDecode(t)
	if s.NextScheduledReset(now).Event.Source.PostID != "200" {
		t.Error("mixed schedule not chosen")
	}
	if sum := s.NextScheduledResetSummary(now); sum == nil || sum.ResetType != ResetBanked {
		t.Errorf("summary = %+v", sum)
	}
}

func TestManualCompletionMergesScheduleTypes(t *testing.T) {
	now := at(t, "2026-07-28T12:00:00Z")
	global := newEvent("reset_scheduled", "2026-07-28T08:00:00Z", rtype("global"), effective("2026-07-28T09:00:00Z"), post("100"))
	banked := newEvent("reset_scheduled", "2026-07-28T08:30:00Z", rtype("banked"), effective("2026-07-28T09:00:00Z"), post("200"))
	manual := `{"id": "manual:mixed", "completedAt": "2026-07-28T10:00:00Z", "visibleUntil": "2026-08-07T10:00:00Z",
      "representativePostId": "100", "schedulePostIds": ["100", "200"],
      "schedules": [` + global.JSON() + `, ` + banked.JSON() + `], "fulfillmentOrigin": "manual"}`
	s := newFeed(now).withTimeline(timelineJSON("", "", "", manual, "")).mustDecode(t)
	if d := s.DisplayResetType(now, time.UTC); d == nil || *d != ResetGlobalAndBanked {
		t.Errorf("display = %v", d)
	}
	if l := s.LatestReset(now); l == nil || l.ResetType != ResetGlobalAndBanked {
		t.Errorf("latest = %+v", l)
	}
}
