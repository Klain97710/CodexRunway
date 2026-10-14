package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/estimate"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

func TestPanelDetailScrollKeepsHeaderVisibleAndReturnsToMainPosition(t *testing.T) {
	snap := uiReviewDetails(panelFixture())
	snap.Codex.Estimate = &estimate.Snapshot{CycleStartDate: "2026-10-05", UsedPercent: 31, UsedCredits: 620}
	for i := 0; i < 30; i++ {
		snap.Codex.Estimate.CurrentRows = append(snap.Codex.Estimate.CurrentRows, estimate.Row{Date: time.Date(2026, 9, 13+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), Credits: 20, CreditsReported: true, Tokens: 1000, TotalsReported: true})
	}
	a, tt := newPanel(t, nil, snap, PanelActions{})
	tt.SetSize(400, 504)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	initialAccount, ok := tt.Find("Ada Lovelace")
	if !ok || initialAccount.H <= 0 {
		t.Fatal("initial overview identity is missing")
	}
	title := a.L().Text(l10n.KeyQuotaEstimateThisWeek)
	for i := 0; i < 20; i++ {
		r, ok := tt.Find(title)
		if ok && r.Y >= 70 && r.Y+r.H < 460 {
			break
		}
		tt.Scroll(200, 300, 0, 60)
	}
	estimate, ok := tt.Find(title)
	if !ok || estimate.Y < 70 || estimate.Y+estimate.H >= 460 {
		t.Fatalf("estimate could not be reached by real scrolling: %+v", estimate)
	}
	mainY, mainOffset := estimate.Y, a.panelState.mainScroll.Y
	if mainOffset <= 0 {
		t.Fatal("real wheel events did not scroll the overview")
	}
	if err := tt.Click(title); err != nil {
		t.Fatal(err)
	}
	if a.panelState.detail != store.ModuleQuotaEstimate {
		t.Fatal("real section click did not open detail")
	}
	back := a.L().Text(l10n.KeyBack)
	header, ok := tt.Find(back)
	if !ok || header.W <= 0 || header.H <= 0 || header.Y < 0 || header.Y+header.H > 150 {
		t.Fatalf("detail inherited overview offset: Back at %+v, found=%v", header, ok)
	}
	metric, metricFound := tt.Find(a.L().Text(l10n.KeyQuotaEstimateWeeklyAllowance))
	if !metricFound || metric.W <= 0 || metric.H <= 0 || metric.Y < header.Y+header.H || metric.Y+metric.H > 300 {
		t.Fatalf("first detail metrics are clipped at entry: %+v", metric)
	}
	if a.panelState.detailScroll.Y != 0 || a.panelState.mainScroll.Y != mainOffset {
		t.Fatal("detail did not start at top independently of overview")
	}
	t.Logf("detail entered at top with visible metric %+v, overview offset %v", metric, mainOffset)
	tt.Scroll(200, 300, 0, 240)
	metricAfter, _ := tt.Find(a.L().Text(l10n.KeyQuotaEstimateWeeklyAllowance))
	if a.panelState.detailScroll.Y <= 0 || a.panelState.mainScroll.Y != mainOffset {
		t.Fatal("real detail scroll did not move independently")
	}
	t.Logf("detail scrolled to %v, metric clipped %+v", a.panelState.detailScroll.Y, metricAfter)
	after, ok := tt.Find(back)
	if !ok || after.W <= 0 || after.H <= 0 || after.Y != header.Y || after.Y+after.H > 150 {
		t.Fatalf("Back scrolled away with detail content: before=%+v after=%+v", header, after)
	}
	if err := tt.Click(back); err != nil {
		t.Fatal(err)
	}
	if a.panelState.detail != "" {
		t.Fatal("visible Back button did not return")
	}
	restored, ok := tt.Find(title)
	if !ok || restored.Y != mainY || a.panelState.mainScroll.Y != mainOffset {
		t.Fatalf("overview lost scroll position: before=%v after=%+v", mainY, restored)
	}
	a.panelClosed()
	reopened := ui.NewTester(a.panelView, 400, 504)
	account, ok := reopened.Find("Ada Lovelace")
	if !ok || account.W <= 0 || account.H <= 0 || account.Y != initialAccount.Y || account.Y+account.H > 300 || a.panelState.mainScroll.Y != 0 || a.panelState.detailScroll.Y != 0 {
		t.Fatalf("reopened overview retained old scrolling: %+v", account)
	}
	a.settings.Flush()
}
