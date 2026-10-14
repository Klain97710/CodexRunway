package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/estimate"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

func TestPanelParitySubscriptionBelongsToCurrentAccount(t *testing.T) {
	snap := panelFixture()
	snap.Accounts = runway.AccountsView{ActiveID: "current", Sidebar: []codex.ManagedAccount{
		{ID: "current", DisplayName: "Current fixture", PlanType: "plus"},
		{ID: "other", DisplayName: "Other fixture", PlanType: "team"},
	}}
	a, tt := newPanel(t, nil, snap, PanelActions{})
	a.panelState.detail = detailAccounts
	tt.Frame()
	current, _ := tt.Find("Current fixture")
	other, _ := tt.Find("Other fixture")
	tier, _ := tt.Find("Plus")
	if tier.Y < current.Y || tier.Y+tier.H > other.Y {
		t.Fatalf("current subscription appears under another account: current=%v tier=%v other=%v; texts %q", current, tier, other, tt.Texts())
	}
}

func TestPanelParityEstimateSeparatesMetricsAndDailyColumns(t *testing.T) {
	l := l10n.New(l10n.English)
	credits, usd := 2000.0, 80.0
	s := &estimate.Snapshot{EstimatedCredits: &credits, EstimatedUSD: &usd, UsedPercent: 31, CycleStartDate: "2026-10-05", CalculatedAt: time.Now(), CurrentRows: []estimate.Row{{Date: "2026-10-12", Credits: 130, CreditsReported: true, Tokens: 65000, TotalsReported: true}}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { estimateDetail(c, l, runway.CodexView{Estimate: s}) })
	}, 368, 1000)
	for _, text := range []string{l.Text(l10n.KeyQuotaEstimateWeeklyAllowance), l.Text(l10n.KeyQuotaEstimateThisWeekUsed), l.Text(l10n.KeyQuotaEstimateUsedPercent), l.Text(l10n.KeyQuotaEstimateWeekValue), l.Text(l10n.KeyDate), l.Text(l10n.KeyTokens), "130.000", "65.00K"} {
		if !hasExact(tt, text) {
			t.Fatalf("estimate metric or daily column not distinct: %q; texts %q", text, tt.Texts())
		}
	}
	date, _ := tt.Find("2026-10-12")
	credit, _ := tt.Find("130.000")
	tokens, _ := tt.Find("65.00K")
	if date.Y != credit.Y || credit.Y != tokens.Y || date.X+date.W > credit.X || credit.X+credit.W > tokens.X || tokens.X+tokens.W > 368 {
		t.Fatalf("daily columns overlap or exceed panel: date=%v credits=%v tokens=%v", date, credit, tokens)
	}
}
