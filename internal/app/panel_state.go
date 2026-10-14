package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

// panelState is ephemeral view state, owned by App on the UI thread.
type panelState struct {
	provider          store.Provider
	detail            store.Module
	scrollDetail      store.Module
	mainScroll        ui.ScrollState
	detailScroll      ui.ScrollState
	chartMode         usage.HeatmapMode
	account           string
	accountGeneration uint64
	cost              panelCostState
	accounts          accountsState
	reaction          panelReactionState
	info              string // id of the open (i) popover
	// setChartStyle saves the chart style; panelView points it at Settings.
	setChartStyle func(store.ChartStyle)
	// startReaction runs the reaction task; panelView points it at App.
	startReaction func(PanelActions, bool)
}

// detailAccounts is the multi-account page. It is not a module, but it is
// shown the way the module detail pages are.
const detailAccounts store.Module = "accounts"

func (s *panelState) mode() usage.HeatmapMode {
	switch s.chartMode {
	case usage.HeatmapWeekly, usage.HeatmapCumulative:
		return s.chartMode
	default:
		return usage.HeatmapDaily
	}
}

func (s *panelState) show(m store.Module) func() { return func() { s.detail = m } }
