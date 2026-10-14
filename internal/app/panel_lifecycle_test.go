package app

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestPanelCloseReopenStartsFreshPresentation(t *testing.T) {
	a, tt := newPanel(t, nil, panelFixture(), PanelActions{})
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	if err := tt.Click(a.L().Text(l10n.KeyHeatmapWeekly)); err != nil {
		t.Fatal(err)
	}
	// The chart lives on the main page; its title is not a detail link.
	if a.panelState.detail != "" || a.panelState.mode() != usage.HeatmapWeekly {
		t.Fatal("weekly selection did not update the main-page chart")
	}
	// Also leave a real detail page open, so closing must reset both states.
	if err := tt.Click(a.L().Text(l10n.KeyAccounts)); err != nil {
		t.Fatal(err)
	}
	if a.panelState.detail != detailAccounts || !tt.HasText(a.L().Text(l10n.KeyBack)) {
		t.Fatal("account identity did not open its detail page")
	}
	provider, account := a.panelState.provider, a.panelState.account
	a.panelClosed() // The actual OnClosed handler, without creating a native window.
	reopened := ui.NewTester(a.panelView, panelWidth, 860)
	if a.panelState.detail != "" || a.panelState.mode() != usage.HeatmapDaily {
		t.Fatal("closing retained detail or chart mode")
	}
	if !reopened.HasText("Ada Lovelace") || reopened.HasText(a.L().Text(l10n.KeyBack)) {
		t.Fatal("reopen did not restore the summary")
	}
	if a.panelState.provider != provider || a.panelState.account != account {
		t.Fatal("closing changed the selected identity")
	}
}

func TestPanelClosedCancelsDetailRequests(t *testing.T) {
	a, _ := newPanel(t, nil, panelFixture(), PanelActions{})
	cost, cancelCost := context.WithCancel(context.Background())
	a.panelState.cost = panelCostState{cancel: cancelCost, generation: 3, task: Task{Running: true}}
	a.panelState.detail = store.ModuleAPICost
	a.panelClosed()
	if cost.Err() == nil {
		t.Fatal("close did not cancel requests")
	}
	if a.panelState.cost.generation != 4 {
		t.Fatal("close lost request generations")
	}
	if a.panelState.cost.task.Running {
		t.Fatal("close retained a running task")
	}
}
