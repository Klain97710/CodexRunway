package resettoday

import (
	"math"
	"time"

	"github.com/licoy/quosparr/internal/alerts"
)

// State is the resolved yes/no/unknown answer (RateLimitResetTodayState).
type State string

// States.
const (
	StateYes     State = "yes"
	StateNo      State = "no"
	StateUnknown State = "unknown"
)

// MonitorStatus is the producer's health.
type MonitorStatus string

// Monitor statuses.
const (
	MonitorOK       MonitorStatus = "ok"
	MonitorDegraded MonitorStatus = "degraded"
)

// Monitor is the feed's monitor block.
type Monitor struct {
	Status    MonitorStatus
	ErrorCode *string
}

// EventKind is the kind of a feed event.
type EventKind string

// Event kinds; the raw values are the feed's.
const (
	KindResetCompleted EventKind = "reset_completed"
	KindResetScheduled EventKind = "reset_scheduled"
	KindBankedReset    EventKind = "banked_reset"
	KindLimitIncrease  EventKind = "limit_increase"
	KindUncertain      EventKind = "uncertain"
)

// ResetType and ScheduleBasis are the alerts package types, so the alert
// producer and the verdict share one definition.
type (
	ResetType     = alerts.ResetType
	ScheduleBasis = alerts.ScheduleBasis
)

// Reset types and schedule bases.
const (
	ResetGlobal          = alerts.ResetGlobal
	ResetBanked          = alerts.ResetBanked
	ResetGlobalAndBanked = alerts.ResetGlobalAndBanked

	BasisExplicit            = alerts.BasisExplicit
	BasisContextualInference = alerts.BasisContextualInference
)

// SchedulePrecision says whether a schedule names a day or an instant.
type SchedulePrecision string

// Schedule precisions; "" means the feed omitted it (legacy feeds).
const (
	PrecisionNone     SchedulePrecision = ""
	PrecisionDate     SchedulePrecision = "date"
	PrecisionDatetime SchedulePrecision = "datetime"
)

// Scope lists the plans and windows an event applies to.
type Scope struct {
	Plans   []string
	Windows []string
}

// Origin says who published an event.
type Origin string

// Origins.
const (
	OriginX        Origin = "x"
	OriginOperator Origin = "operator"
)

// EventSource is the Swift RateLimitResetTodaySource (renamed so it does not
// read like export.ResetTodaySource).
type EventSource struct {
	Origin Origin
	Handle *string
	PostID string
	URL    *string
}

// IsOperator reports whether the operator confirmed the event by hand.
func (s EventSource) IsOperator() bool { return s.Origin == OriginOperator }

// IsXPostID reports whether value is a numeric X post ID (1 to 30 numbers).
func IsXPostID(value string) bool {
	if value == "" {
		return false
	}
	count := 0
	for _, r := range value {
		if !isNumber(r) {
			return false
		}
		count++
	}
	return count <= 30
}

// IsOperatorEventID reports whether value is "op_" plus 24 lowercase hex.
func IsOperatorEventID(value string) bool {
	if len(value) != 27 || value[:3] != "op_" {
		return false
	}
	for i := 3; i < len(value); i++ {
		c := value[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// IsEventID reports whether value is an X post ID or an operator event ID.
func IsEventID(value string) bool { return IsXPostID(value) || IsOperatorEventID(value) }

// Event is one feed event.
type Event struct {
	Kind               EventKind
	ResetType          ResetType
	AnnouncedAt        time.Time
	EffectiveAt        *time.Time
	SchedulePrecision  SchedulePrecision
	ScheduleBasis      ScheduleBasis // "" when absent
	ScheduleGraceHours *float64
	Scope              Scope
	Source             EventSource
	Confidence         float64
	Rationale          string
	// Text is the original post text. It is validated, never shown.
	Text string
}

// ScheduleGrace and StaleAfter are the Swift constants.
const (
	ScheduleGrace = 3 * time.Hour
	StaleAfter    = 30 * time.Hour
)

func (e Event) graceInterval() float64 {
	if e.ScheduleGraceHours != nil {
		return *e.ScheduleGraceHours * 3600
	}
	return ScheduleGrace.Seconds()
}

// graceDeadline is pendingUntil plus the grace interval; false when either is
// not a finite number.
func (e Event) graceDeadline(pendingUntil time.Time) (time.Time, bool) {
	interval := e.graceInterval()
	if math.IsNaN(interval) || math.IsInf(interval, 0) {
		return time.Time{}, false
	}
	return addSeconds(pendingUntil, interval), true
}

// resetOccurrenceAt is when a completed reset took effect; nil for every
// other kind.
func (e Event) resetOccurrenceAt() *time.Time {
	if e.Kind != KindResetCompleted {
		return nil
	}
	if e.EffectiveAt != nil {
		return e.EffectiveAt
	}
	at := e.AnnouncedAt
	return &at
}

// maxOffsetSeconds keeps absurd but finite grace values inside the range a
// time.Time can hold; Swift's Date takes any finite double.
const maxOffsetSeconds = 1e17

func addSeconds(t time.Time, seconds float64) time.Time {
	seconds = math.Max(-maxOffsetSeconds, math.Min(maxOffsetSeconds, seconds))
	whole := math.Floor(seconds)
	nanos := int64(math.Round((seconds - whole) * 1e9))
	return time.Unix(t.Unix()+int64(whole), int64(t.Nanosecond())+nanos).UTC()
}

// ScheduleWindow is when a scheduled reset may land.
type ScheduleWindow struct {
	StartAt time.Time
	EndAt   time.Time
	IsRange bool
}

// PendingUntil: display ranges end at 23:59, lifecycle validity ends at the
// next midnight.
func (w ScheduleWindow) PendingUntil() time.Time {
	if w.IsRange {
		return w.EndAt.Add(time.Minute)
	}
	return w.EndAt
}

// PublishedWindow is the window the feed recorded for a fulfilled schedule.
type PublishedWindow struct {
	StartAt   time.Time
	EndAt     time.Time
	Precision SchedulePrecision
}

// FulfilledSchedule is a schedule the feed marked as completed.
type FulfilledSchedule struct {
	Schedule          Event
	Window            PublishedWindow
	CompletionPostID  string
	CompletedAt       time.Time
	VisibleUntil      time.Time
	FulfillmentOrigin *string
	ClusterID         *string
}

// FulfilledBanked is a banked reset the feed marked as completed.
type FulfilledBanked struct {
	Banked           Event
	CompletionPostID string
	CompletedAt      time.Time
	VisibleUntil     time.Time
}

// ManualCompletion is a reset the operator confirmed by hand.
type ManualCompletion struct {
	ID                   string
	CompletedAt          time.Time
	VisibleUntil         time.Time
	RepresentativePostID string
	SchedulePostIDs      []string
	Schedules            []Event
	FulfillmentOrigin    string
}

// RepresentativeEvent is the schedule named by RepresentativePostID, else the
// first one.
func (m ManualCompletion) RepresentativeEvent() *Event {
	for i := range m.Schedules {
		if m.Schedules[i].Source.PostID == m.RepresentativePostID {
			return &m.Schedules[i]
		}
	}
	if len(m.Schedules) > 0 {
		return &m.Schedules[0]
	}
	return nil
}

// ResetType merges the schedules' types; global when there are none.
func (m ManualCompletion) ResetType() ResetType {
	types := make([]ResetType, len(m.Schedules))
	for i, s := range m.Schedules {
		types[i] = s.ResetType
	}
	if t := alerts.MergeResetTypes(types); t != "" {
		return t
	}
	return ResetGlobal
}

// Timeline is the authoritative reset timeline of newer feeds.
type Timeline struct {
	NextSchedule             *Event
	RecentNonCompletedPostID *string
	FulfilledSchedules       []FulfilledSchedule
	FulfilledBanked          []FulfilledBanked
	ManualCompletions        []ManualCompletion
	SuppressedPostIDs        []string
}

// PendingReset is a scheduled reset that has not lapsed yet.
type PendingReset struct {
	EffectiveAt    time.Time
	EffectiveUntil time.Time
	IsRange        bool
	Event          Event
}

// Window is the schedule window of the pending reset.
func (p PendingReset) Window() ScheduleWindow {
	return ScheduleWindow{StartAt: p.EffectiveAt, EndAt: p.EffectiveUntil, IsRange: p.IsRange}
}

func mergeTypes(types []ResetType) (ResetType, bool) {
	t := alerts.MergeResetTypes(types)
	return t, t != ""
}
