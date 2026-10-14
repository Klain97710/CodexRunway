package codexapi

import (
	"sort"
	"time"
)

// RateWindow is one quota window. Zero ResetsAt / WindowMinutes mean the API
// did not say.
type RateWindow struct {
	// UsedPercent is the rounded percentage; UsedPercentExact keeps the value
	// as sent.
	UsedPercent      int
	UsedPercentExact float64
	WindowMinutes    int
	ResetsAt         time.Time
}

// NamedRateWindow is an additional (model specific) limit.
type NamedRateWindow struct {
	Name   string
	Window RateWindow
}

// QuotaSnapshot is the decoded usage response: the 5 hour window (Primary),
// the weekly one (Secondary, when present), additional limits and credits.
type QuotaSnapshot struct {
	Plan              string
	Primary           RateWindow
	Secondary         *RateWindow
	AdditionalWindows []NamedRateWindow
	// CreditsBalance is nil when the response had no readable balance.
	CreditsBalance *float64
	UpdatedAt      time.Time
}

// NextDueReset returns the earliest reset after triggered that passed at least
// one second ago, or the zero time. triggered may be zero.
func (q QuotaSnapshot) NextDueReset(triggered, now time.Time) time.Time {
	resets := []time.Time{q.Primary.ResetsAt}
	if q.Secondary != nil {
		resets = append(resets, q.Secondary.ResetsAt)
	}
	for _, w := range q.AdditionalWindows {
		resets = append(resets, w.Window.ResetsAt)
	}
	var due []time.Time
	for _, r := range resets {
		if !r.IsZero() && r.After(triggered) && now.Sub(r) >= time.Second {
			due = append(due, r)
		}
	}
	if len(due) == 0 {
		return time.Time{}
	}
	sort.Slice(due, func(i, j int) bool { return due[i].Before(due[j]) })
	return due[0]
}

// CycleAnchorWindow is the longest complete window, the billing/usage cycle
// used for API cost (usually the weekly one).
func (q QuotaSnapshot) CycleAnchorWindow() (RateWindow, bool) {
	candidates := []RateWindow{}
	if q.Secondary != nil {
		candidates = append(candidates, *q.Secondary)
	}
	candidates = append(candidates, q.Primary)
	for _, w := range q.AdditionalWindows {
		candidates = append(candidates, w.Window)
	}
	var best RateWindow
	found := false
	for _, w := range candidates {
		if w.ResetsAt.IsZero() || w.WindowMinutes <= 0 {
			continue
		}
		if !found || best.WindowMinutes < w.WindowMinutes {
			best, found = w, true
		}
	}
	return best, found
}

// Interval is a half-open time span.
type Interval struct{ Start, End time.Time }

// CycleWindows returns the full cycle and the part elapsed up to now.
func (q QuotaSnapshot) CycleWindows(now time.Time) (full, elapsed Interval, ok bool) {
	anchor, found := q.CycleAnchorWindow()
	if !found {
		return Interval{}, Interval{}, false
	}
	start := anchor.ResetsAt.Add(-time.Duration(anchor.WindowMinutes) * time.Minute)
	if !start.Before(anchor.ResetsAt) {
		return Interval{}, Interval{}, false
	}
	full = Interval{start, anchor.ResetsAt}
	end := now
	if end.Before(start) {
		end = start
	}
	if end.After(anchor.ResetsAt) {
		end = anchor.ResetsAt
	}
	// Zero-length windows produce empty scans and false "unavailable" states.
	if !end.After(start) {
		end = start.Add(time.Second)
		if end.After(anchor.ResetsAt) {
			end = anchor.ResetsAt
		}
	}
	return full, Interval{start, end}, true
}

// ResetCredit is one reset credit. Zero times mean unknown.
type ResetCredit struct {
	ID        string
	Status    string
	CreatedAt time.Time
	ExpiresAt time.Time
	// RemainingSeconds is the time left at decode time, 0 when unknown or past.
	RemainingSeconds float64
}

// ResetCreditsSnapshot is the decoded reset credit response.
type ResetCreditsSnapshot struct {
	AvailableCount int
	Credits        []ResetCredit
	UpdatedAt      time.Time
}

// TokenUsageProfile is the daily token series of the Codex profile endpoint
// (CodexProfileTokenUsage). Keys of DailyTokens are UTC "YYYY-MM-DD" days.
type TokenUsageProfile struct {
	DailyTokens map[string]int
	StatsAsOf   string
	// GeneratedAt is zero when absent.
	GeneratedAt time.Time
}
