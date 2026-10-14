package resettoday

import (
	"testing"
	"time"
)

// Direct-construction helpers (the Swift makeSnapshot/scheduled/completed
// private helpers of the verdict tests).

type snapOpts struct {
	events                []Event
	graceSchedules        []Event
	timeline              *Timeline
	generatedAt           *time.Time
	lastSuccessfulCheckAt *time.Time
	monitor               *Monitor
}

func makeSnapshot(now time.Time, o snapOpts) *Snapshot {
	generated := now
	if o.generatedAt != nil {
		generated = *o.generatedAt
	}
	last := now
	if o.lastSuccessfulCheckAt != nil {
		last = *o.lastSuccessfulCheckAt
	}
	m := Monitor{Status: MonitorOK}
	if o.monitor != nil {
		m = *o.monitor
	}
	return newSnapshot(response{
		schemaVersion:         1,
		generatedAt:           generated,
		lastSuccessfulCheckAt: &last,
		monitor:               m,
		events:                append([]Event{}, o.events...),
		graceSchedules:        append([]Event{}, o.graceSchedules...),
		resetTimeline:         o.timeline,
	}, now, time.UTC)
}

func scheduledEvent(when time.Time, confidence float64, postID string, t ResetType) Event {
	eff := when
	return Event{
		Kind:              KindResetScheduled,
		ResetType:         t,
		AnnouncedAt:       when.Add(-time.Hour),
		EffectiveAt:       &eff,
		SchedulePrecision: PrecisionDatetime,
		ScheduleBasis:     BasisExplicit,
		Scope:             Scope{Plans: []string{"all"}, Windows: []string{"weekly"}},
		Source:            EventSource{Origin: OriginX, PostID: postID},
		Confidence:        confidence,
		Rationale:         "Explicit Codex quota reset schedule.",
		Text:              "Scheduled reset.",
	}
}

func completedEvent(when time.Time, confidence float64, postID string) Event {
	eff := when
	return Event{
		Kind:        KindResetCompleted,
		ResetType:   ResetGlobal,
		AnnouncedAt: when,
		EffectiveAt: &eff,
		Scope:       Scope{Plans: []string{"all"}, Windows: []string{"weekly"}},
		Source:      EventSource{Origin: OriginX, PostID: postID},
		Confidence:  confidence,
		Rationale:   "Explicit Codex quota reset announcement.",
		Text:        "Reset completed.",
	}
}

func mustDate(t testing.TB, s string) time.Time { return at(t, s) }
