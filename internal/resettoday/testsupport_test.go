package resettoday

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
)

// The fixture builders below port RateLimitResetTodayTestSupport.swift: they
// write feed JSON text (not Go structs), so the decoder is what each test
// exercises, exactly as the Swift tests do.

type eventFixture struct {
	Kind              string
	ResetType         string
	AnnouncedAt       string
	EffectiveAt       string // "" renders null
	SchedulePrecision string
	ScheduleBasis     string
	GraceHoursJSON    string
	PostID            string
	Origin            string
	OmitOrigin        bool
	Handle            string
	NoHandle          bool
	NoURL             bool
}

type eventOption func(*eventFixture)

func newEvent(kind, announcedAt string, opts ...eventOption) eventFixture {
	e := eventFixture{Kind: kind, AnnouncedAt: announcedAt, PostID: "123", Handle: "thsottiaux"}
	for _, o := range opts {
		o(&e)
	}
	return e
}

func effective(v string) eventOption  { return func(e *eventFixture) { e.EffectiveAt = v } }
func precision(v string) eventOption  { return func(e *eventFixture) { e.SchedulePrecision = v } }
func basis(v string) eventOption      { return func(e *eventFixture) { e.ScheduleBasis = v } }
func rtype(v string) eventOption      { return func(e *eventFixture) { e.ResetType = v } }
func post(v string) eventOption       { return func(e *eventFixture) { e.PostID = v } }
func graceHours(v string) eventOption { return func(e *eventFixture) { e.GraceHoursJSON = v } }
func origin(v string) eventOption     { return func(e *eventFixture) { e.Origin = v } }
func noOrigin() eventOption           { return func(e *eventFixture) { e.OmitOrigin = true } }
func handle(v string) eventOption     { return func(e *eventFixture) { e.Handle, e.NoHandle = v, false } }
func withURL() eventOption            { return func(e *eventFixture) { e.NoURL = false } }
func noHandle() eventOption           { return func(e *eventFixture) { e.NoHandle = true } }
func noURL() eventOption              { return func(e *eventFixture) { e.NoURL = true } }

// operatorEvent is an operator-confirmed event with the given op_ id.
func operatorEvent(kind, announcedAt, id string, opts ...eventOption) eventFixture {
	all := append([]eventOption{post(id), origin("operator"), noHandle(), noURL()}, opts...)
	return newEvent(kind, announcedAt, all...)
}

func (e eventFixture) rationale() string {
	pick := func(global, banked, both string) string {
		switch e.ResetType {
		case "banked":
			return banked
		case "global_and_banked":
			return both
		}
		return global
	}
	switch e.Kind {
	case "reset_completed":
		if e.Origin == "operator" {
			return pick("Operator-confirmed Codex quota reset without an X announcement.",
				"Operator-confirmed Codex reset-bank credit without an X announcement.",
				"Operator-confirmed Codex global reset and reset-bank credit without an X announcement.")
		}
		return pick("Explicit Codex quota reset announcement.",
			"Explicit Codex reset-bank credit announcement.",
			"Explicit Codex global reset and reset-bank credit announcement.")
	case "reset_scheduled":
		switch {
		case e.Origin == "operator":
			return pick("Operator-confirmed Codex quota reset schedule without an X announcement.",
				"Operator-confirmed Codex reset-bank credit schedule without an X announcement.",
				"Operator-confirmed Codex global reset and reset-bank credit schedule without an X announcement.")
		case e.ScheduleBasis == "contextual_inference":
			return pick("High-probability Codex quota reset preview inferred from context.",
				"High-probability Codex reset-bank credit preview inferred from context.",
				"High-probability Codex global reset and reset-bank credit preview inferred from context.")
		}
		return pick("Explicit Codex quota reset schedule.",
			"Explicit Codex reset-bank credit schedule.",
			"Explicit Codex global reset and reset-bank credit schedule.")
	case "banked_reset":
		return "Banked reset announcement; not a completed reset."
	case "limit_increase":
		return "Quota limit increase announcement; not a reset."
	}
	return "Not a clear reset signal."
}

func (e eventFixture) JSON() string {
	effectiveValue := "null"
	if e.EffectiveAt != "" {
		effectiveValue = `"` + e.EffectiveAt + `"`
	}
	extra := ""
	if e.ResetType != "" {
		extra += fmt.Sprintf(",\n          \"resetType\": %q", e.ResetType)
	}
	if e.SchedulePrecision != "" {
		extra += fmt.Sprintf(",\n          \"schedulePrecision\": %q", e.SchedulePrecision)
	}
	if e.ScheduleBasis != "" {
		extra += fmt.Sprintf(",\n          \"scheduleBasis\": %q", e.ScheduleBasis)
	}
	if e.GraceHoursJSON != "" {
		extra += ",\n          \"scheduleGraceHours\": " + e.GraceHoursJSON
	}
	source := ""
	if e.Origin != "" && !e.OmitOrigin {
		source += fmt.Sprintf("\n            \"origin\": %q,", e.Origin)
	}
	if !e.NoHandle {
		source += fmt.Sprintf("\n            \"handle\": %q,", e.Handle)
	}
	source += fmt.Sprintf("\n            \"postId\": %q", e.PostID)
	if !e.NoURL {
		source += fmt.Sprintf(",\n            \"url\": \"https://x.com/thsottiaux/status/%s\"", e.PostID)
	}
	return fmt.Sprintf(`{
          "kind": %q,
          "announcedAt": %q,
          "effectiveAt": %s%s,
          "scope": {"plans": ["all"], "windows": ["weekly"]},
          "source": {%s
          },
          "confidence": 0.98,
          "rationale": %q,
          "text": "fixture post text for %s"
        }`, e.Kind, e.AnnouncedAt, effectiveValue, extra, source, e.rationale(), e.Kind)
}

func eventsJSON(events ...eventFixture) string {
	parts := make([]string, len(events))
	for i, e := range events {
		parts[i] = e.JSON()
	}
	return strings.Join(parts, ",")
}

// feedFixture ports ResetStatusFeedFixture. Its zero-value defaults are set by
// newFeed.
type feedFixture struct {
	EventsJSON            string
	GraceSchedulesJSON    string // "" omits the key
	ResetTimelineJSON     string // "" omits the key
	GeneratedAt           string
	LastSuccessfulCheckAt string // "" renders null
	MonitorStatus         string
	ErrorCode             string // "" renders null
	Now                   time.Time
	Loc                   *time.Location
}

func newFeed(now time.Time, events ...eventFixture) feedFixture {
	return feedFixture{
		EventsJSON:            eventsJSON(events...),
		GeneratedAt:           "2026-07-28T12:00:00Z",
		LastSuccessfulCheckAt: "2026-07-28T12:00:00Z",
		MonitorStatus:         "ok",
		Now:                   now,
		Loc:                   time.UTC,
	}
}

func (f feedFixture) checked(v string) feedFixture       { f.LastSuccessfulCheckAt = v; return f }
func (f feedFixture) generated(v string) feedFixture     { f.GeneratedAt = v; return f }
func (f feedFixture) using(l *time.Location) feedFixture { f.Loc = l; return f }
func (f feedFixture) monitor(status, code string) feedFixture {
	f.MonitorStatus, f.ErrorCode = status, code
	return f
}
func (f feedFixture) withTimeline(j string) feedFixture { f.ResetTimelineJSON = j; return f }
func (f feedFixture) withGrace(events ...eventFixture) feedFixture {
	f.GraceSchedulesJSON = eventsJSON(events...)
	return f
}

func (f feedFixture) JSON() string {
	lastSuccess := "null"
	if f.LastSuccessfulCheckAt != "" {
		lastSuccess = `"` + f.LastSuccessfulCheckAt + `"`
	}
	errorValue := "null"
	if f.ErrorCode != "" {
		errorValue = `"` + f.ErrorCode + `"`
	}
	timeline, grace := "", ""
	if f.ResetTimelineJSON != "" {
		timeline = ",\n          \"resetTimeline\": " + f.ResetTimelineJSON
	}
	if f.GraceSchedulesJSON != "" {
		grace = ",\n          \"graceSchedules\": [" + f.GraceSchedulesJSON + "]"
	}
	return fmt.Sprintf(`{
          "schemaVersion": 1,
          "generatedAt": %q,
          "lastSuccessfulCheckAt": %s,
          "monitor": {"status": %q, "errorCode": %s},
          "events": [%s]%s%s
        }`, f.GeneratedAt, lastSuccess, f.MonitorStatus, errorValue, f.EventsJSON, grace, timeline)
}

func (f feedFixture) decode() (*Snapshot, error) {
	return Decode([]byte(f.JSON()), f.Now, f.Loc)
}

func (f feedFixture) mustDecode(t testing.TB) *Snapshot {
	t.Helper()
	s, err := f.decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

// timelineJSON ports resetStatusEmptyTimelineJSON.
func timelineJSON(nextSchedule, recentNonCompleted, fulfilled, manual, suppressed string) string {
	next := "null"
	if nextSchedule != "" {
		next = nextSchedule
	}
	recent := "null"
	if recentNonCompleted != "" {
		recent = `"` + recentNonCompleted + `"`
	}
	extra := ""
	if manual != "" {
		extra += ",\n          \"manualCompletions\": [" + manual + "]"
	}
	if suppressed != "" {
		extra += ",\n          \"suppressedPostIds\": [" + suppressed + "]"
	}
	return fmt.Sprintf(`{
      "nextSchedule": %s,
      "recentNonCompletedPostId": %s,
      "fulfilledSchedules": [%s]%s
    }`, next, recent, fulfilled, extra)
}

func emptyTimeline() string { return timelineJSON("", "", "", "", "") }

func at(t testing.TB, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("bad test date %q: %v", s, err)
	}
	return v.UTC()
}

func mustLoc(t testing.TB, name string) *time.Location {
	t.Helper()
	l, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("zone %s: %v", name, err)
	}
	return l
}

var (
	english = l10n.New(l10n.English)
	chinese = l10n.New(l10n.SimplifiedChinese)
)

func ptr[T any](v T) *T { return &v }

// wantErr checks that decoding failed with a DecodeError or any error.
func wantDecodeError(t *testing.T, f feedFixture) {
	t.Helper()
	if s, err := f.decode(); err == nil {
		t.Fatalf("decode succeeded (state %v), want an error", s.State())
	}
}
