package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

// providerView reuses Codex display widgets with the same typed Grok data.
func providerView(l l10n.Localizer, snap runway.Snapshot, displayRange store.CostRange) runway.CodexView {
	if snap.Provider != store.ProviderGrok {
		return snap.Codex
	}
	g := snap.Grok
	v := runway.CodexView{Meters: g.Meters, Heatmap: g.Heatmap, DisplayedCost: g.Cost, DisplayedRange: displayRange, Cost: g.Cost, ResetCredits: g.ResetCredits, ResetCreditSummary: g.ResetCreditSummary, RecentSessions: g.RecentSessions}
	if g.Cost != nil {
		d := runway.DisplayCost(l, *g.Cost, displayRange, "", time.Now())
		v.CostText, v.CostSubtitle, v.CostLines = d.Text, d.Subtitle, d.Lines
	}
	if g.ResetCredits != nil {
		v.CreditDetails = runway.CreditDetails(l, *g.ResetCredits)
	}
	return v
}

// panelDetail draws the scrolling body of the reset credits and quota
// estimate pages. The accounts and API cost pages lay themselves out.
func panelDetail(c *ui.Context, l l10n.Localizer, m store.Module, v runway.CodexView) {
	switch m {
	case store.ModuleResetCredits:
		creditsDetail(c, l, v)
	case store.ModuleQuotaEstimate:
		estimateDetail(c, l, v)
	}
}
