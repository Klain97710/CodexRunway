package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/estimate"
	"github.com/licoy/quosparr/internal/resettoday"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestExportDetailFlows(t *testing.T) {
	if os.Getenv("RUNWAY_UI_QA") == "" {
		t.Skip("set RUNWAY_UI_QA to export detail PNGs")
	}
	queue := detailMainQueue(t)
	dir := filepath.Join("..", "..", ".build", "v1-ui-qa")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []store.Language{store.LanguageEnglish, store.LanguageSimplifiedChinese} {
		snap := uiReviewDetails(uiReviewSnapshot())
		estimated, change := 2000.0, 12.5
		previous := estimate.HistorySample{CycleStartDate: "2026-09-28", EstimatedCredits: 1777.8, UsedPercent: 72.4, UsedCredits: 1287.1, RecordedAt: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)}
		snap.Codex.Estimate = &estimate.Snapshot{CycleStartDate: "2026-10-05", UsedCredits: 620, UsedPercent: 31, EstimatedCredits: &estimated, CanExtrapolate: true, CalculatedAt: time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC), PreviousEstimate: &previous, ChangePercent: &change, History: []estimate.HistorySample{previous, {CycleStartDate: "2026-09-21", EstimatedCredits: 1740, UsedPercent: 80, RecordedAt: previous.RecordedAt.AddDate(0, 0, -7)}}, CurrentRows: []estimate.Row{{Date: "2026-10-10", Credits: 130, CreditsReported: true, Tokens: 65000}, {Date: "2026-10-11", Credits: 200, CreditsReported: true, Tokens: 88000}, {Date: "2026-10-12", Credits: 290, CreditsReported: true, Tokens: 122000}}}
		sum := *snap.Codex.DisplayedCost
		acts := PanelActions{QueryCost: func(_ context.Context, _ store.Provider, q CostQuery) (usage.Summary, error) {
			copy := sum
			if q.Custom != nil {
				copy.Window = q.Custom.Window
			}
			return copy, nil
		}}
		a, tt := newPanel(t, func(p *store.Preferences) { p.Language = lang }, snap, acts)
		tt.SetSize(400, 584)
		a.panelState.detail = store.ModuleAPICost
		tt.Frame()
		finishDetail(t, queue)
		tt.Frame()
		a.panelState.cost.selected = 4
		a.panelState.cost.start = time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
		a.panelState.cost.end = time.Date(2026, 10, 12, 0, 0, 0, 0, time.Local)
		r, _ := usage.CustomRange(a.panelState.cost.start, a.panelState.cost.end, time.Local)
		a.startCostQuery(CostQuery{Custom: &r}, acts)
		finishDetail(t, queue)
		tt.Frame()
		exportReviewPNG(t, tt, filepath.Join(dir, "after-cost-range-"+string(lang)+".png"))
		a.cancelPanelDetails()
		a.panelState.detail = store.ModuleQuotaEstimate
		tt.Frame()
		exportReviewPNG(t, tt, filepath.Join(dir, "after-estimate-history-"+string(lang)+".png"))
		a.panelState.detail = store.ModuleRateLimitResetToday
		a.panelState.reaction.initialized = true
		reaction := resettoday.ReactionDevMock(resettoday.MockYes, 321)
		a.panelState.reaction.snapshot = &reaction
		tt.Frame()
		exportReviewPNG(t, tt, filepath.Join(dir, "after-reset-reaction-"+string(lang)+".png"))
	}
}
