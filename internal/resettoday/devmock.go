package resettoday

import (
	"strings"
	"time"
)

// DevMockKind selects a local fixture instead of the network.
type DevMockKind string

// Dev mock kinds.
const (
	MockYes               DevMockKind = "yes"
	MockNo                DevMockKind = "no"
	MockScheduled         DevMockKind = "scheduled"
	MockUnknown           DevMockKind = "unknown"
	MockCompleted         DevMockKind = "completed"
	MockExplicitScheduled DevMockKind = "explicitScheduled"
	MockInferredScheduled DevMockKind = "inferredScheduled"
	MockGrace             DevMockKind = "grace"
	MockExpired           DevMockKind = "expired"
	MockUnavailable       DevMockKind = "unavailable"
)

// ParseDevMockKind accepts the aliases of `--mock-reset-today=`.
func ParseDevMockKind(raw string) (DevMockKind, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "yes", "y", "1", "true":
		return MockYes, true
	case "no", "n", "0", "false":
		return MockNo, true
	case "scheduled", "schedule":
		return MockScheduled, true
	case "unknown":
		return MockUnknown, true
	case "completed":
		return MockCompleted, true
	case "explicit-scheduled", "explicit_scheduled":
		return MockExplicitScheduled, true
	case "inferred-scheduled", "inferred_scheduled", "inferred":
		return MockInferredScheduled, true
	case "grace":
		return MockGrace, true
	case "expired":
		return MockExpired, true
	case "unavailable":
		return MockUnavailable, true
	}
	return "", false
}

const mockFlag = "--mock-reset-today="

// ResolveDevMockKind reads `--mock-reset-today=<kind>` from args, else the
// QUOSPARR_MOCK_RESET_TODAY environment value. The first such flag wins
// even when its value is unknown (then there is no mock).
func ResolveDevMockKind(args []string, getenv func(string) string) (DevMockKind, bool) {
	for _, a := range args {
		if strings.HasPrefix(a, mockFlag) {
			return ParseDevMockKind(strings.TrimPrefix(a, mockFlag))
		}
	}
	if getenv != nil {
		if v := getenv("QUOSPARR_MOCK_RESET_TODAY"); v != "" {
			return ParseDevMockKind(v)
		}
	}
	return "", false
}

// DevMockForState is devMock(state:now:).
func DevMockForState(state State, now time.Time, loc *time.Location) *Snapshot {
	kind := MockUnknown
	switch state {
	case StateYes:
		kind = MockYes
	case StateNo:
		kind = MockNo
	}
	return DevMock(kind, now, loc)
}

// DevMock is devMock(kind:now:): a feed shaped like the real one, built
// around now.
func DevMock(kind DevMockKind, now time.Time, loc *time.Location) *Snapshot {
	unavailable := kind == MockUnknown || kind == MockUnavailable
	monitor := Monitor{Status: MonitorOK}
	if unavailable {
		code := "request_failed"
		monitor = Monitor{Status: MonitorDegraded, ErrorCode: &code}
	}
	now = now.UTC()
	events := []Event{}
	switch kind {
	case MockYes, MockCompleted:
		events = []Event{mockEvent(KindResetCompleted, now, nil, PrecisionNone, "", "2079433708986319143", 0.98,
			"Explicit Codex quota reset announcement.", "I have reset usage limits for Codex.", []string{"weekly", "five_hour"})}
	case MockScheduled, MockExplicitScheduled, MockInferredScheduled, MockGrace, MockExpired:
		src := sourceLocation()
		var effective time.Time
		switch kind {
		case MockGrace:
			effective = now.Add(-time.Hour)
		case MockExpired:
			effective = now.Add(-4 * time.Hour)
		default:
			effective = addDay(startOfDay(now, src), src).UTC()
		}
		announced := now
		if a := effective.Add(-time.Hour); a.Before(announced) {
			announced = a
		}
		precision := PrecisionDatetime
		if kind == MockScheduled {
			precision = PrecisionDate
		}
		basis := BasisExplicit
		if kind == MockInferredScheduled {
			basis = BasisContextualInference
		}
		events = []Event{mockEvent(KindResetScheduled, announced, &effective, precision, basis, "2086189414292865249", 0.92,
			"Explicit Codex quota reset schedule.", "I'll do another reset tomorrow.", []string{"weekly"})}
	}
	r := response{
		schemaVersion:  1,
		generatedAt:    now,
		monitor:        monitor,
		events:         events,
		graceSchedules: []Event{},
	}
	if !unavailable {
		at := now
		r.lastSuccessfulCheckAt = &at
	}
	return newSnapshot(r, now, loc)
}

func mockEvent(kind EventKind, announced time.Time, effective *time.Time, precision SchedulePrecision,
	basis ScheduleBasis, postID string, confidence float64, rationale, text string, windows []string) Event {
	handle := "thsottiaux"
	u := "https://x.com/thsottiaux/status/" + postID
	return Event{
		Kind:              kind,
		ResetType:         ResetGlobal,
		AnnouncedAt:       announced.UTC(),
		EffectiveAt:       effective,
		SchedulePrecision: precision,
		ScheduleBasis:     basis,
		Scope:             Scope{Plans: []string{"all"}, Windows: windows},
		Source:            EventSource{Origin: OriginX, Handle: &handle, PostID: postID, URL: &u},
		Confidence:        confidence,
		Rationale:         rationale,
		Text:              text,
	}
}
