package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

// panelClaudeQuota shows the 5-hour window, the 7-day window and the
// per-model weekly windows. Extra usage keeps the raw numbers and currency.
func panelClaudeQuota(c *ui.Context, l l10n.Localizer, snap runway.Snapshot, acts PanelActions, first *bool, st *panelState, rs resetSwitch) {
	g := snap.Claude
	refreshing := g.Refreshing || snap.Refresh.Has(runway.SectionQuota)
	spec := sectionSpec{title: l.Text(l10n.KeyProviderClaude), refreshing: refreshing, refresh: sectionRefresh(acts, runway.SectionQuota)}
	sectionBlock(c, first, func() {
		drawSectionHead(c, l, st, spec)
		if g.Gateway && g.Availability != runway.ClaudeAPIKeyMode {
			dimText(c, l.Text(l10n.KeyClaudeGatewayNotice))
		}
		if len(g.Meters) == 0 {
			if g.LastError != "" {
				calloutDim(c, g.LastError)
			} else {
				sectionEmpty(c, l, refreshing)
			}
			return
		}
		ui.Column(c).Gap(12).Children(func() {
			now := time.Now()
			for i, m := range g.Meters {
				grokMeterRow(c, l, m, i == 0, now, rs)
			}
			if g.ExtraUsage != "" {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, l.Text(l10n.KeyClaudeExtraUsage)).FontSize(fsCaption).FontWeight(600)
					ui.Text(c, g.ExtraUsage).FontSize(fsCaption).FontFeatures("tnum")
				})
			}
		})
	})
}
