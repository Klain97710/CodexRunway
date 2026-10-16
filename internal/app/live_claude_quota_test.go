package app

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

func TestClaudeKindPercentQuotaReachesMetersAccountsAndWidget(t *testing.T) {
	raw := json.RawMessage(`{
		"five_hour":{"utilization":3},
		"seven_day":{"utilization":72},
		"limits":[
			{"kind":"session","percent":3,"is_active":false,"resets_at":"2026-10-15T06:30:00Z"},
			{"kind":"weekly_all","percent":72,"is_active":true,"resets_at":"2026-10-17T19:00:00Z"}
		]
	}`)
	snap, err := claude.ParseUsageSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	l := l10n.New(l10n.English)
	at := time.Date(2026, 10, 15, 1, 42, 0, 0, time.UTC)
	meters := claudeMeters(l, snap, at)
	if len(meters) != 2 || meters[0].RemainingPercent != 97 || meters[1].RemainingPercent != 28 {
		t.Fatalf("main panel lost quota: %+v", meters)
	}
	if meters[0].WindowMinutes != runway.FiveHourMinutes || meters[1].WindowMinutes != runway.WeeklyMinutes {
		t.Fatalf("wrong quota windows: %+v", meters)
	}
	accounts := claudeAccountQuotas(l, []claude.ManagedAccount{{
		ID: "one", SubscriptionType: "pro", CachedQuota: raw, LastQuotaAt: &at,
	}})
	q, ok := accounts["one"]
	if !ok || q.Title == "" || q.UsedPercent != 3 || !q.UpdatedAt.Equal(at) || !q.ResetsAt.Equal(meters[0].ResetsAt) {
		t.Fatalf("account summary lost quota: %+v", accounts)
	}
	widget := claudeWidget(runway.ClaudeState{Meters: meters, UpdatedAt: at})
	if len(widget.Quota) != 2 || widget.Quota[0].RemainingPercent != 97 || widget.Quota[1].RemainingPercent != 28 {
		t.Fatalf("widget lost quota: %+v", widget.Quota)
	}
}

func TestClaudeResetGrantsReachPanelAndWidget(t *testing.T) {
	raw := json.RawMessage(`{
		"five_hour":{"utilization":40},
		"cedar_ember":{"eligible":true,"grants":[
			{"id":"weekly","resets_total":2,"resets_left":1,"ends_at":"2026-10-18T00:00:00Z"}
		]}
	}`)
	snap, err := claude.ParseUsageSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	b := &claudeBridge{state: runway.ClaudeState{Availability: runway.ClaudeReady, UpdatedAt: at, Detail: &snap}}
	st := b.State(at)
	if st.ResetCreditSummary == nil || st.ResetCreditSummary.AvailableCount != 1 || st.ResetCreditSummary.TotalCount != 2 ||
		st.ResetCreditSummary.ExpiringCount != 1 || !st.ResetCreditSummary.UpdatedAt.Equal(at) {
		t.Fatalf("summary: %+v", st.ResetCreditSummary)
	}
	if st.Widget.ResetCredits == nil {
		t.Fatal("widget lost reset credits")
	}
	v := claudeAsCodexView(l10n.New(l10n.English), st, "")
	if v.ResetCreditSummary == nil || len(v.CreditDetails) != 2 {
		t.Fatalf("panel view: %+v", v.CreditDetails)
	}
	snap.ResetGrants = &claude.ResetGrantStatus{Eligible: false}
	if st := b.State(at); st.ResetCreditSummary != nil || st.Widget.ResetCredits != nil {
		t.Fatal("widget of an ineligible account without grants carries reset credits")
	}
	// The panel keeps the module at zero for such accounts and API accounts.
	for _, st := range []runway.ClaudeState{b.State(at), {API: &runway.ClaudeAPIView{}}} {
		v := claudeAsCodexView(l10n.New(l10n.English), st, "")
		if v.ResetCreditSummary == nil || v.ResetCreditSummary.AvailableCount != 0 || v.ResetCreditSummary.TotalCount != 0 {
			t.Fatalf("panel hides zero reset credits: %+v", v.ResetCreditSummary)
		}
	}
	if got := runway.CreditUpdatedText(time.Time{}, l10n.English, nil); got != "--" {
		t.Fatal(got)
	}
}
