package runwayfmt

import (
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/resettoday"
)

const completedFeed = `{
  "schemaVersion": 1,
  "generatedAt": "2026-07-28T12:00:00Z",
  "lastSuccessfulCheckAt": "2026-07-28T12:00:00Z",
  "monitor": {"status": "ok", "errorCode": null},
  "events": [{
    "kind": "reset_completed",
    "announcedAt": "2026-07-28T04:09:02Z",
    "effectiveAt": "2026-07-28T04:09:02Z",
    "scope": {"plans": ["all"], "windows": ["weekly"]},
    "source": {"handle": "thsottiaux", "postId": "1", "url": "https://x.com/thsottiaux/status/1"},
    "confidence": 0.98,
    "rationale": "Explicit Codex quota reset announcement.",
    "text": "Reset completed."
  }]
}`

const scheduledFeed = `{
  "schemaVersion": 1,
  "generatedAt": "2026-07-28T12:00:00Z",
  "lastSuccessfulCheckAt": "2026-07-28T12:00:00Z",
  "monitor": {"status": "ok", "errorCode": null},
  "events": [{
    "kind": "reset_scheduled",
    "announcedAt": "2026-07-28T09:00:00Z",
    "effectiveAt": "2026-07-28T13:00:00Z",
    "schedulePrecision": "datetime",
    "scope": {"plans": ["all"], "windows": ["weekly"]},
    "source": {"handle": "thsottiaux", "postId": "2", "url": "https://x.com/thsottiaux/status/2"},
    "confidence": 0.92,
    "rationale": "Explicit Codex quota reset schedule.",
    "text": "Scheduled reset."
  }]
}`

func decode(t *testing.T, feed string, now time.Time) *resettoday.Snapshot {
	t.Helper()
	s, err := resettoday.Decode([]byte(feed), now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestConfirmedDetailMatchesHostedCopy(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	s := decode(t, completedFeed, now)
	l := l10n.New(l10n.SimplifiedChinese)
	d := s.VerdictDetail(l, Formatter{}, now, time.UTC)
	if d == nil {
		t.Fatal("no detail")
	}
	if d.TypeLabel != "全局重置" || d.PercentText == nil || *d.PercentText != "98%" {
		t.Fatalf("detail = %+v", d)
	}
	if d.TimeText == nil || d.PlainText == "" {
		t.Fatalf("missing time/plain: %+v", d)
	}
	if want := "监测到今天已有完成的全局重置，置信度约为98%，重置的本地时间为" + *d.TimeText; d.PlainText != want {
		t.Fatalf("plain = %q, want %q", d.PlainText, want)
	}
	for _, kind := range []resettoday.TokenKind{resettoday.TokenResetType, resettoday.TokenPercent, resettoday.TokenTime} {
		if !hasToken(d, kind) {
			t.Errorf("missing token kind %v", kind)
		}
	}
}

func TestScheduledChanceDetailMatchesHostedCopy(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	s := decode(t, scheduledFeed, now)
	d := s.VerdictDetail(l10n.New(l10n.SimplifiedChinese), Formatter{}, now, time.UTC)
	if d == nil {
		t.Fatal("no detail")
	}
	if d.PercentText == nil || *d.PercentText != "≥92%" || d.TimeText == nil || *d.TimeText != "2026/7/28 13:00" {
		t.Fatalf("detail = %+v", d)
	}
	const want = "约≥92%的可能性会进行全局重置，目前计划已排期，重置时间范围大约在本地时间：2026/7/28 13:00"
	if d.PlainText != want {
		t.Fatalf("plain = %q, want %q", d.PlainText, want)
	}
	if !hasToken(d, resettoday.TokenResetType) || !hasToken(d, resettoday.TokenTime) || hasToken(d, resettoday.TokenPercent) {
		t.Fatalf("tokens = %+v", d.Tokens)
	}
}

func hasToken(d *resettoday.DetailPresentation, kind resettoday.TokenKind) bool {
	for _, tok := range d.Tokens {
		if tok.Kind == kind {
			return true
		}
	}
	return false
}

func TestNoneHintNamesLastResetTypeAndSingleUnitAgo(t *testing.T) {
	// The completed reset is from the previous local day, 20h21m earlier.
	now := time.Date(2026, 7, 29, 0, 30, 0, 0, time.UTC)
	s := decode(t, completedFeed, now)
	zh := l10n.New(l10n.SimplifiedChinese)
	hint := s.NoneHintLastReset(zh, Formatter{}, now)
	if hint == nil {
		t.Fatal("no hint")
	}
	if hint.ResetType != resettoday.ResetGlobal || hint.Ago != "20小时" ||
		hint.Text != "今日暂无已完成或已排期的重置，上次发生全局重置为20小时之前" {
		t.Fatalf("hint = %+v", *hint)
	}
	want := []resettoday.NoneSegment{
		{Kind: resettoday.SegmentText, Text: "今日暂无已完成或已排期的重置，上次发生"},
		{Kind: resettoday.SegmentResetType},
		{Kind: resettoday.SegmentText, Text: "为"},
		{Kind: resettoday.SegmentAgo},
		{Kind: resettoday.SegmentText, Text: "之前"},
	}
	got := resettoday.NoneHintSegments(zh.Text(l10n.KeyRateLimitResetTodayNoHintWithLast))
	if len(got) != len(want) {
		t.Fatalf("segments = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("segment %d = %+v want %+v", i, got[i], want[i])
		}
	}
}
