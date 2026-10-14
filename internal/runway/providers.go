package runway

import (
	"context"
	"time"

	"github.com/licoy/quosparr/internal/alerts"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/usage"
)

// DetailLine is one "title: value" row of a panel section.
type DetailLine struct {
	Title string
	Value string
}

// GrokAvailability is the state of the Grok panel (Swift GrokPanelAvailability).
type GrokAvailability int

// GrokAvailability values.
const (
	GrokLoading GrokAvailability = iota
	GrokReady
	GrokNotLoggedIn
	GrokReauthRequired
	GrokCLIUnavailable
	GrokCLITooOld
	GrokBillingParseFailed
	GrokFailed
)

// GrokState is what the coordinator needs to know about Grok to build the
// status bar and the widget snapshot. The Grok package owns the rest; Detail
// carries its panel payload untouched for the Grok UI to type-assert.
type GrokState struct {
	Availability GrokAvailability
	Accounts     AccountsView
	// AccountQuotas is the cached quota line of each managed account by id,
	// for the account list; accounts without a reading are absent.
	AccountQuotas map[string]GrokAccountQuota
	// Plan and UpdatedAt describe the quota reading; zero when none.
	Plan      string
	UpdatedAt time.Time
	// Meters are the quota bars, the overall included quota first.
	Meters     []Meter
	LastError  string
	Refreshing bool
	// Widget is the ready-made widget provider block, nil when Grok is unused.
	Widget             *export.ProviderSnapshot
	Detail             any
	Cost               *usage.Summary
	Heatmap            HeatmapData
	RecentSessions     []sessions.Item
	ResetCredits       *codexapi.ResetCreditsSnapshot
	ResetCreditSummary *ResetCreditSummary
}

// GrokAccountQuota is the cached quota of one managed Grok account: the
// included-quota meter and the plan. Title is the localized period name,
// IncludedUSD the "$used / $limit" allowance or "".
type GrokAccountQuota struct {
	Plan        string
	Title       string
	UsedPercent int
	ResetsAt    time.Time
	IncludedUSD string
	UpdatedAt   time.Time
}

// GrokProvider is the seam the Grok package plugs into. The Service never
// imports Grok code: an adapter in the application shell implements this.
//
// Locking: State is called while the Service holds its own lock, so it must
// be quick and must not call back into the Service. The onChange callback
// registered with SetOnChange must be invoked WITHOUT holding any lock that
// State takes, or the two will deadlock.
type GrokProvider interface {
	// State returns the current immutable state as of now.
	State(now time.Time) GrokState
	// Refresh refetches Grok data; it must honour ctx and return when done.
	Refresh(ctx context.Context) error
	// SetOnChange registers the callback invoked after State changed.
	SetOnChange(fn func())
}

// ResetTodayState is the display state of the "rate limit reset today" card.
type ResetTodayState struct {
	// Text is the one-line summary, Lines the detail rows.
	Text  string
	Lines []DetailLine
	// Widget is the widget block, nil without a successful reading.
	Widget *export.ResetTodaySnapshot
	// HasData reports whether a reading exists.
	HasData bool
	// Detail carries the reset-today package payload for its UI.
	Detail any
}

// ResetTodayProvider is the seam of the reset-today feature. The locking
// rules of GrokProvider apply: State must not block or call into the Service,
// and onChange must be called without holding State's lock.
type ResetTodayProvider interface {
	State(now time.Time) ResetTodayState
	// Refresh fetches the status; force skips the provider's own throttling.
	Refresh(ctx context.Context, force bool) error
	// LastFetch is when Refresh last finished (success or failure), zero if never.
	LastFetch() time.Time
	// PendingAlerts returns the alerts that are due at now; the Service
	// dedupes them through the alert store.
	PendingAlerts(now time.Time) []alerts.Alert
	SetOnChange(fn func())
}
