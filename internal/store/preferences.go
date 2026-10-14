// Package store is the compatibility layer for Quosparr's user preferences
// and network proxy settings.
//
// The Swift app keeps both as JSON Data inside its UserDefaults domain
// (com.quosparr.app). The Go app reads and writes the same bytes through
// the system API on macOS, and through JSON files in ~/.quosparr on
// Windows and Linux. The JSON schema, key names, enum values, defaults and
// normalization rules are a contract with the Swift app and must not drift.
//
// Neither value holds a secret: the proxy password lives in the system
// credential store and is only referenced by an opaque id.
package store

import (
	"encoding/json"
	"maps"
	"math"
	"slices"
)

// Defaults and limits shared with the Swift app.
const (
	DefaultRefreshIntervalSeconds                    = 300
	DefaultWidgetRefreshIntervalSeconds              = 60
	MinWidgetRefreshIntervalSeconds                  = 60
	MaxWidgetRefreshIntervalSeconds                  = 1_800
	DefaultRateLimitResetTodayRefreshIntervalSeconds = 300
	MinRateLimitResetTodayRefreshIntervalSeconds     = 300
	MaxRateLimitResetTodayRefreshIntervalSeconds     = 21_600
	DefaultMainPanelHeight                           = 584.0
	MinMainPanelHeight                               = 360.0
	MaxMainPanelHeight                               = 900.0
)

// WidgetRefreshIntervalOptions are the intervals the settings UI offers for
// the widget. Stored values outside the list are kept as long as they are in
// range, exactly as the Swift app does.
func WidgetRefreshIntervalOptions() []int { return []int{60, 300, 600, 900, 1_800} }

// RateLimitResetTodayRefreshIntervalOptions are the intervals the settings UI
// offers for the reset-today module.
func RateLimitResetTodayRefreshIntervalOptions() []int {
	return []int{300, 600, 900, 1_800, 3_600, 7_200, 21_600}
}

// Preferences is the general settings document (UserDefaults key
// "runway.preferences"). The json tags are the on-disk key names. Use
// DecodePreferences and EncodePreferences to read and write the stored form:
// they keep unknown keys and reject what the Swift app would reject.
type Preferences struct {
	SelectedProvider                          Provider               `json:"selectedProvider"`
	Language                                  Language               `json:"language"`
	Appearance                                Appearance             `json:"appearance"`
	MainPanelBackgroundStyle                  PanelBackground        `json:"mainPanelBackgroundStyle"`
	StatusBarDisplayStyle                     StatusBarStyle         `json:"statusBarDisplayStyle"`
	StatusBarMetersDetailStyle                MetersDetail           `json:"statusBarMetersDetailStyle"`
	StatusBarBatteryScope                     BatteryScope           `json:"statusBarBatteryScope"`
	StatusBarBatteryDetailStyle               BatteryDetail          `json:"statusBarBatteryDetailStyle"`
	StatusBarProviderScope                    StatusBarProviderScope `json:"statusBarProviderScope"`
	RefreshIntervalSeconds                    int                    `json:"refreshIntervalSeconds"`
	WidgetRefreshIntervalSeconds              int                    `json:"widgetRefreshIntervalSeconds"`
	APICostSummaryRange                       CostRange              `json:"apiCostSummaryRange"`
	MainPanelHeight                           float64                `json:"mainPanelHeight"`
	MainPanelModuleOrder                      []Module               `json:"mainPanelModuleOrder"`
	ShowsQuotaSummary                         bool                   `json:"showsQuotaSummary"`
	ShowsResetCreditsSummary                  bool                   `json:"showsResetCreditsSummary"`
	ShowsQuotaEstimateSummary                 bool                   `json:"showsQuotaEstimateSummary"`
	QuotaEstimateWindowMode                   WindowMode             `json:"quotaEstimateWindowMode"`
	ShowsCostSummary                          bool                   `json:"showsCostSummary"`
	ShowsRecentSessions                       bool                   `json:"showsRecentSessions"`
	ShowsSessionRepairSummary                 bool                   `json:"showsSessionRepairSummary"`
	ShowsRateLimitResetToday                  bool                   `json:"showsRateLimitResetToday"`
	ShowsModelSpecificQuotaUsage              bool                   `json:"showsModelSpecificQuotaUsage"`
	ShowsTokenUsageHeatmap                    bool                   `json:"showsTokenUsageHeatmap"`
	TokenUsageChartStyle                      ChartStyle             `json:"tokenUsageChartStyle"`
	RateLimitResetTodayRefreshIntervalSeconds int                    `json:"rateLimitResetTodayRefreshIntervalSeconds"`
	AutomaticallyChecksForUpdates             bool                   `json:"automaticallyChecksForUpdates"`
	LaunchAtLoginEnabled                      bool                   `json:"launchAtLoginEnabled"`
	LaunchAtLoginInitialized                  bool                   `json:"launchAtLoginInitialized"`
	QuotaAlertsEnabled                        bool                   `json:"quotaAlertsEnabled"`
	ResetCreditAlertsEnabled                  bool                   `json:"resetCreditAlertsEnabled"`
	RateLimitResetTodayAlertsEnabled          bool                   `json:"rateLimitResetTodayAlertsEnabled"`
	ExportsStatusJSON                         bool                   `json:"exportsStatusJSON"`
	AccountPrivacyModeEnabled                 bool                   `json:"accountPrivacyModeEnabled"`

	// extra holds keys this version does not know, written back unchanged so
	// that older and newer apps sharing the domain do not erase each other's
	// settings.
	extra map[string]json.RawMessage
}

// DefaultPreferences returns the settings of a first launch. It matches the
// Swift RunwayPreferences() initializer.
func DefaultPreferences() Preferences {
	return Preferences{
		SelectedProvider:                          ProviderCodex,
		Language:                                  LanguageSystem,
		Appearance:                                AppearanceSystem,
		MainPanelBackgroundStyle:                  PanelBackgroundTranslucent,
		StatusBarDisplayStyle:                     StatusBarStyleMeters,
		StatusBarMetersDetailStyle:                MetersDetailRemainingPercent,
		StatusBarBatteryScope:                     BatteryScopeFiveHour,
		StatusBarBatteryDetailStyle:               BatteryDetailCountdown,
		StatusBarProviderScope:                    ProviderScopeSelected,
		RefreshIntervalSeconds:                    DefaultRefreshIntervalSeconds,
		WidgetRefreshIntervalSeconds:              DefaultWidgetRefreshIntervalSeconds,
		APICostSummaryRange:                       CostRangeToday,
		MainPanelHeight:                           DefaultMainPanelHeight,
		MainPanelModuleOrder:                      Modules(),
		ShowsQuotaSummary:                         true,
		ShowsResetCreditsSummary:                  true,
		ShowsQuotaEstimateSummary:                 true,
		QuotaEstimateWindowMode:                   WindowModeAuto,
		ShowsCostSummary:                          true,
		ShowsRecentSessions:                       false,
		ShowsSessionRepairSummary:                 true,
		ShowsRateLimitResetToday:                  true,
		ShowsModelSpecificQuotaUsage:              false,
		ShowsTokenUsageHeatmap:                    true,
		TokenUsageChartStyle:                      ChartStyleHeatmap,
		RateLimitResetTodayRefreshIntervalSeconds: DefaultRateLimitResetTodayRefreshIntervalSeconds,
		AutomaticallyChecksForUpdates:             true,
		LaunchAtLoginEnabled:                      true,
		LaunchAtLoginInitialized:                  false,
		QuotaAlertsEnabled:                        false,
		ResetCreditAlertsEnabled:                  false,
		RateLimitResetTodayAlertsEnabled:          true,
		ExportsStatusJSON:                         false,
		AccountPrivacyModeEnabled:                 false,
	}
}

// Clone returns a deep copy.
func (p Preferences) Clone() Preferences {
	p.MainPanelModuleOrder = slices.Clone(p.MainPanelModuleOrder)
	p.extra = maps.Clone(p.extra)
	return p
}

// Normalize applies the clamping and module-order repair the Swift initializer
// and decoder apply. It does not touch enum fields; see Validate.
func (p *Preferences) Normalize() {
	p.WidgetRefreshIntervalSeconds = ClampWidgetRefreshInterval(p.WidgetRefreshIntervalSeconds)
	p.MainPanelHeight = ClampMainPanelHeight(p.MainPanelHeight)
	p.MainPanelModuleOrder = NormalizeModuleOrder(p.MainPanelModuleOrder)
	p.RateLimitResetTodayRefreshIntervalSeconds =
		ClampRateLimitResetTodayRefreshInterval(p.RateLimitResetTodayRefreshIntervalSeconds)
}

// ClampWidgetRefreshInterval limits the widget refresh interval to 60...1800 s.
func ClampWidgetRefreshInterval(seconds int) int {
	return max(MinWidgetRefreshIntervalSeconds, min(MaxWidgetRefreshIntervalSeconds, seconds))
}

// ClampRateLimitResetTodayRefreshInterval limits the interval to 300...21600 s.
func ClampRateLimitResetTodayRefreshInterval(seconds int) int {
	return max(MinRateLimitResetTodayRefreshIntervalSeconds,
		min(MaxRateLimitResetTodayRefreshIntervalSeconds, seconds))
}

// ClampMainPanelHeight limits the height to 360...900; a non-finite value
// becomes the default.
func ClampMainPanelHeight(height float64) float64 {
	if math.IsNaN(height) || math.IsInf(height, 0) {
		return DefaultMainPanelHeight
	}
	return max(MinMainPanelHeight, min(MaxMainPanelHeight, height))
}

// NormalizeModuleOrder drops duplicates and unknown modules, keeps the user's order, and puts each
// missing module back next to its default-order neighbour: just before the
// first later default module that is present, or at the end.
func NormalizeModuleOrder(order []Module) []Module {
	result := make([]Module, 0, len(Modules()))
	for _, m := range order {
		if m.Valid() && !slices.Contains(result, m) {
			result = append(result, m)
		}
	}
	defaults := Modules()
	for i, m := range defaults {
		if slices.Contains(result, m) {
			continue
		}
		result = insertBeforeNeighbor(result, m, defaults[i+1:])
	}
	return result
}

// insertBeforeNeighbor places m before the first of later that is in order, or
// appends it when none is.
func insertBeforeNeighbor(order []Module, m Module, later []Module) []Module {
	for _, n := range later {
		if at := slices.Index(order, n); at >= 0 {
			return slices.Insert(order, at, m)
		}
	}
	return append(order, m)
}

// MoveMainPanelModule moves m by offset positions (swapping with the module
// at the destination). It does nothing when m or the destination is missing.
func (p *Preferences) MoveMainPanelModule(m Module, offset int) {
	order := NormalizeModuleOrder(p.MainPanelModuleOrder)
	from := slices.Index(order, m)
	to := from + offset
	if from < 0 || to < 0 || to >= len(order) {
		return
	}
	order[from], order[to] = order[to], order[from]
	p.MainPanelModuleOrder = order
}

// ResetMainPanelModuleOrder restores the default module order.
func (p *Preferences) ResetMainPanelModuleOrder() { p.MainPanelModuleOrder = Modules() }
