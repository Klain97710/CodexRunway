package runway

import (
	"slices"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// maxFloatingMeters is how many quota windows the floating window shows for
// one provider.
const maxFloatingMeters = 2

// FloatingMeter is one quota window of the floating window.
type FloatingMeter struct {
	// Label is the short window name ("5h", "Weekly").
	Label            string
	RemainingPercent int
	Health           Health
	// Reset is the localized time left until the reset, "" without one.
	Reset string
}

// FloatingProvider is one provider of the floating window. Without meters,
// Status says why ("Log in", "Wait").
type FloatingProvider struct {
	Provider store.Provider
	Name     string
	Meters   []FloatingMeter
	Status   string
}

// FloatingView is the data side of the floating window.
type FloatingView struct {
	Providers []FloatingProvider
}

// BuildFloating computes the floating window content for the preferences.
func BuildFloating(lc l10n.Localizer, in StatusInputs) FloatingView {
	var v FloatingView
	for _, p := range in.Prefs.FloatingSelection() {
		item := FloatingProvider{Provider: p, Name: lc.Text(providerStatusKey(p))}
		for _, m := range floatingMeters(metersOf(in, p), preferredOf(p)) {
			fm := FloatingMeter{Label: ShortWindowTitle(lc, m.WindowMinutes), RemainingPercent: m.RemainingPercent, Health: m.Health}
			if m.HasReset() {
				fm.Reset = DurationLocalized(m.ResetsAt.Sub(in.Now).Seconds(), lc.Language(), false)
			}
			item.Meters = append(item.Meters, fm)
		}
		if len(item.Meters) == 0 {
			item.Status = floatingStatus(lc, in, p)
		}
		v.Providers = append(v.Providers, item)
	}
	return v
}

// floatingMeters picks up to two standard windows, the preferred lengths
// first; a provider with only model-specific windows shows its first one.
func floatingMeters(meters []Meter, preferred []int) []Meter {
	var standard []Meter
	for _, m := range meters {
		if m.Source == SourceStandard {
			standard = append(standard, m)
		}
	}
	if len(standard) == 0 {
		return meters[:min(1, len(meters))]
	}
	rank := func(m Meter) int {
		if i := slices.Index(preferred, m.WindowMinutes); i >= 0 {
			return i
		}
		return len(preferred)
	}
	slices.SortStableFunc(standard, func(a, b Meter) int { return rank(a) - rank(b) })
	return standard[:min(maxFloatingMeters, len(standard))]
}

// floatingStatus is the provider's status text when it has no reading. The
// Codex text may be a countdown, which says nothing without a meter.
func floatingStatus(lc l10n.Localizer, in StatusInputs, p store.Provider) string {
	if p == store.ProviderCodex && len(in.CodexMeters) == 0 && in.CodexText == "" {
		return lc.Text(l10n.KeyStatusWait)
	}
	return statusTextFor(lc, in, p)
}
