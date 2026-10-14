package runway

import (
	"github.com/licoy/quosparr/internal/codex"
	"strconv"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// RenderPlan says which meters the status item shows and how they stack
// (Swift StatusBarRenderPlan).
type RenderPlan struct {
	Meters        []Meter
	RowsPerColumn int
}

// Columns chunks the meters into columns of RowsPerColumn.
func (p RenderPlan) Columns() [][]Meter {
	if p.RowsPerColumn <= 0 {
		return nil
	}
	var cols [][]Meter
	for start := 0; start < len(p.Meters); start += p.RowsPerColumn {
		cols = append(cols, p.Meters[start:min(start+p.RowsPerColumn, len(p.Meters))])
	}
	return cols
}

// MakeRenderPlan filters the meters for the style and picks the row count.
func MakeRenderPlan(style store.StatusBarStyle, scope store.BatteryScope, meters []Meter) RenderPlan {
	visible := meters
	if style == store.StatusBarStyleBattery {
		visible = batteryMeters(scope, meters)
	}
	rows := 1
	if (style == store.StatusBarStyleBattery || style == store.StatusBarStyleMeters) && len(visible) > 1 {
		rows = 2
	}
	return RenderPlan{Meters: visible, RowsPerColumn: rows}
}

func batteryMeters(scope store.BatteryScope, meters []Meter) []Meter {
	if scope == store.BatteryScopeBoth {
		return meters
	}
	var standard, specific []Meter
	for _, m := range meters {
		if m.Source == SourceStandard {
			standard = append(standard, m)
		} else {
			specific = append(specific, m)
		}
	}
	want := WeeklyMinutes
	if scope == store.BatteryScopeFiveHour {
		want = FiveHourMinutes
	}
	var picked *Meter
	for i := range standard {
		if standard[i].WindowMinutes == want {
			picked = &standard[i]
			break
		}
	}
	if picked == nil && len(standard) > 0 {
		picked = &standard[0]
	}
	if picked == nil {
		return specific
	}
	return append([]Meter{*picked}, specific...)
}

// Preferred window lengths when one meter per provider is chosen.
var (
	codexPreferredWindows = []int{FiveHourMinutes, WeeklyMinutes}
	grokPreferredWindows  = []int{WeeklyMinutes, MonthlyMinutes}
)

// PrimaryStandardMeter picks the first standard meter matching a preferred
// window length, else the first standard meter, else the first meter.
func PrimaryStandardMeter(meters []Meter, preferred []int) (Meter, bool) {
	var standard []Meter
	for _, m := range meters {
		if m.Source == SourceStandard {
			standard = append(standard, m)
		}
	}
	if len(standard) == 0 {
		if len(meters) == 0 {
			return Meter{}, false
		}
		return meters[0], true
	}
	for _, minutes := range preferred {
		for _, m := range standard {
			if m.WindowMinutes == minutes {
				return m, true
			}
		}
	}
	return standard[0], true
}

// WithShortTitle retitles a meter with the short window name.
func WithShortTitle(lc l10n.Localizer, m Meter) Meter {
	m.Title = ShortWindowTitle(lc, m.WindowMinutes)
	return m
}

// DualProviderMeters picks one meter per provider, Codex first, titled
// "Codex · 5h" style.
func DualProviderMeters(lc l10n.Localizer, codex, grok []Meter, codexLabel, grokLabel string) []Meter {
	var out []Meter
	if m, ok := PrimaryStandardMeter(codex, codexPreferredWindows); ok {
		m.Title = codexLabel + " · " + ShortWindowTitle(lc, m.WindowMinutes)
		out = append(out, m)
	}
	var g Meter
	ok := false
	if len(grok) > 0 {
		g, ok = PrimaryStandardMeter(grok[:1], grokPreferredWindows)
	}
	if !ok && len(grok) > 0 {
		g, ok = grok[0], true
	}
	if ok {
		g.Title = grokLabel + " · " + ShortWindowTitle(lc, g.WindowMinutes)
		out = append(out, g)
	}
	return out
}

// StatusInputs is everything the status item selection reads.
type StatusInputs struct {
	Prefs    store.Preferences
	Provider store.Provider
	// Codex side.
	CodexTier   codex.Tier
	CodexText   string
	CodexMeters []Meter
	// Grok side; the zero state reads as loading.
	Grok GrokState
	Now  time.Time
}

// GrokStatusText is the status text of Grok: the countdown of the first
// meter, its remaining percentage, or the availability fallback.
func GrokStatusText(lc l10n.Localizer, g GrokState, now time.Time) string {
	if len(g.Meters) == 0 {
		switch g.Availability {
		case GrokNotLoggedIn, GrokReauthRequired:
			return lc.Text(l10n.KeyStatusLogin)
		case GrokLoading:
			return lc.Text(l10n.KeyStatusWait)
		case GrokReady:
			return lc.Text(l10n.KeyStatusUnknown)
		}
		return lc.Text(l10n.KeyStatusError)
	}
	m := g.Meters[0]
	if m.HasReset() {
		return DurationLocalized(m.ResetsAt.Sub(now).Seconds(), lc.Language(), false)
	}
	return strconv.Itoa(m.RemainingPercent) + "%"
}

// SelectedStatusText is the status item text for the chosen provider scope.
func SelectedStatusText(lc l10n.Localizer, in StatusInputs) string {
	if in.Prefs.StatusBarProviderScope == store.ProviderScopeBoth {
		if len(in.CodexMeters) > 0 && in.CodexText != "" {
			return in.CodexText
		}
		return GrokStatusText(lc, in.Grok, in.Now)
	}
	if in.Provider == store.ProviderGrok {
		return GrokStatusText(lc, in.Grok, in.Now)
	}
	return in.CodexText
}

// SelectedMeters are the meters the status item draws.
func SelectedMeters(lc l10n.Localizer, in StatusInputs) []Meter {
	if in.Prefs.StatusBarProviderScope == store.ProviderScopeBoth {
		return DualProviderMeters(lc, in.CodexMeters, in.Grok.Meters,
			lc.Text(l10n.KeyProviderCodex), lc.Text(l10n.KeyProviderGrok))
	}
	if in.Provider == store.ProviderCodex {
		return in.CodexMeters
	}
	if len(in.Grok.Meters) == 0 {
		return nil
	}
	return []Meter{WithShortTitle(lc, in.Grok.Meters[0])}
}

// StatusBarItem is the text data of one meter in the status item.
type StatusBarItem struct {
	Meter Meter
	// Ring is the remaining percentage ("--" without a meter).
	Ring          string
	Countdown     string
	Battery       string
	BatteryDetail string
	MeterText     string
}

// StatusBarView is the data side of the status item: what to draw, no pixels.
type StatusBarView struct {
	Style store.StatusBarStyle
	Text  string
	Plan  RenderPlan
	Items []StatusBarItem
	// TextCaptions are the captions of the "text" style ("NN%" or "--").
	TextCaptions []string
	Tooltip      string
}

// BuildStatusBar computes the status item content for the preferences.
func BuildStatusBar(lc l10n.Localizer, in StatusInputs) StatusBarView {
	if in.CodexTier == codex.TierPro100 || in.CodexTier == codex.TierPro200 || in.CodexTier == codex.TierPro500 {
		visible := make([]Meter, 0, len(in.CodexMeters))
		for _, m := range in.CodexMeters {
			if m.Source != SourceStandard || m.WindowMinutes != FiveHourMinutes {
				visible = append(visible, m)
			}
		}
		in.CodexMeters = visible
	}
	text := SelectedStatusText(lc, in)
	meters := SelectedMeters(lc, in)
	plan := MakeRenderPlan(in.Prefs.StatusBarDisplayStyle, in.Prefs.StatusBarBatteryScope, meters)
	view := StatusBarView{Style: in.Prefs.StatusBarDisplayStyle, Text: text, Plan: plan}
	lang := lc.Language()
	for _, m := range plan.Meters {
		view.Items = append(view.Items, StatusBarItem{
			Meter:         m,
			Ring:          RingText(&m),
			Countdown:     CountdownCaption(m, in.Now, lang),
			Battery:       BatteryCaption(m, in.Prefs.StatusBarBatteryDetailStyle, in.Now, lang),
			BatteryDetail: BatteryDetail(m, in.Prefs.StatusBarBatteryDetailStyle, in.Now, lang),
			MeterText:     MeterCaption(m, in.Prefs.StatusBarMetersDetailStyle, in.Now, lang),
		})
		view.TextCaptions = append(view.TextCaptions, RingText(&m)+"%")
	}
	if len(plan.Meters) == 0 {
		view.TextCaptions = []string{RingText(nil)}
	}
	view.Tooltip = Tooltip(text, meters)
	return view
}

// Tooltip is "Quosparr · text" plus one "title: NN%" line.
func Tooltip(text string, meters []Meter) string {
	head := "Quosparr · " + text
	if len(meters) == 0 {
		return head
	}
	parts := make([]string, len(meters))
	for i, m := range meters {
		parts[i] = m.Title + ": " + strconv.Itoa(m.RemainingPercent) + "%"
	}
	return head + "\n" + strings.Join(parts, " · ")
}

// RingText is the remaining percentage or "--".
func RingText(m *Meter) string {
	if m == nil {
		return "--"
	}
	return strconv.Itoa(m.RemainingPercent)
}

func resetCountdown(m Meter, now time.Time, lang l10n.Language) string {
	if !m.HasReset() {
		return "--"
	}
	return DurationLocalized(m.ResetsAt.Sub(now).Seconds(), lang, false)
}

func titled(m Meter, detail string) string {
	if m.Source == SourceModelSpecific {
		return detail + " " + m.Title
	}
	return m.Title + " " + detail
}

// CountdownCaption is the caption of the "countdown" style.
func CountdownCaption(m Meter, now time.Time, lang l10n.Language) string {
	return titled(m, resetCountdown(m, now, lang))
}

// BatteryDetail is the detail text of the "battery" style.
func BatteryDetail(m Meter, style store.BatteryDetail, now time.Time, lang l10n.Language) string {
	if style == store.BatteryDetailRemainingPercent {
		return strconv.Itoa(m.RemainingPercent) + "%"
	}
	return resetCountdown(m, now, lang)
}

// BatteryCaption is the caption of the "battery" style.
func BatteryCaption(m Meter, style store.BatteryDetail, now time.Time, lang l10n.Language) string {
	return titled(m, BatteryDetail(m, style, now, lang))
}

// MeterDetail is the detail text of the "meters" style.
func MeterDetail(m Meter, style store.MetersDetail, now time.Time, lang l10n.Language) string {
	reset := "--"
	if m.HasReset() {
		reset = ResetShortLabel(m.ResetsAt, now, nil)
	}
	switch style {
	case store.MetersDetailResetTime:
		return reset
	case store.MetersDetailBoth:
		return strconv.Itoa(m.RemainingPercent) + "% · " + reset
	}
	return strconv.Itoa(m.RemainingPercent) + "%"
}

// MeterCaption is the caption of the "meters" style.
func MeterCaption(m Meter, style store.MetersDetail, now time.Time, lang l10n.Language) string {
	return titled(m, MeterDetail(m, style, now, lang))
}
