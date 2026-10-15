package app

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestPanelRuntimeRejectsCompletedDetailAfterSameAccountReapply(t *testing.T) {
	queue := detailMainQueue(t)
	p := quietLivePreferences()
	p.Language = store.LanguageEnglish
	p.ShowsCostSummary = true
	auth := codex.APIKeyAuth("synthetic-account-reapply")
	var switched atomic.Bool
	index := codex.AccountIndex{ActiveAccountID: "ada", Accounts: []codex.ManagedAccount{{ID: "ada", DisplayName: "Synthetic account"}}}
	svc := runway.New(runway.Options{Features: personal.Features{Grok: true, Claude: true}, Preferences: p, Sources: runway.Sources{
		LoadIndex:     func() (codex.AccountIndex, error) { return index, nil },
		LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) { return auth, nil },
		SwitchAccount: func(context.Context, string) (codex.SwitchResult, error) {
			switched.Store(true)
			return codex.SwitchResult{Auth: auth, Account: index.Accounts[0]}, nil
		},
		FetchQuota: func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) {
			return codexapi.QuotaSnapshot{}, nil
		},
		FetchResetCredits: func(context.Context, codexapi.Auth) (codexapi.ResetCreditsSnapshot, error) {
			return codexapi.ResetCreditsSnapshot{}, nil
		},
		ScanCost: func(_ context.Context, queries []usage.Query, _ time.Time, _ usage.RefreshPolicy, _ *usage.ProgressReporter) (map[string]usage.Summary, error) {
			tokens := int64(111)
			if switched.Load() {
				tokens = 222
			}
			out := map[string]usage.Summary{}
			for _, q := range queries {
				out[q.ID] = usage.Summary{Window: q.Window, Totals: usage.Totals{TotalTokens: tokens}}
			}
			return out, nil
		},
	}})
	defer svc.Close()
	svc.ReloadAccounts()
	rt := &liveApp{svc: svc}
	a, tt := newPanel(t, func(prefs *store.Preferences) { *prefs = p }, svc.Snapshot(), PanelActions{})
	a.SetPanelActions(rt.panelActions(a.settings))
	if err := tt.Click(a.L().Text(l10n.KeyShowDetails)); err != nil {
		t.Fatal(err)
	}
	var completed func()
	select {
	case completed = <-queue:
	case <-time.After(time.Second):
		t.Fatal("real service detail query did not complete")
	}
	old := svc.Snapshot()
	if err := svc.SwitchAccount(context.Background(), "ada"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	fresh := svc.Snapshot()
	if fresh.Accounts.ActiveID != old.Accounts.ActiveID || fresh.Codex.Generation == old.Codex.Generation {
		t.Fatal("fixture did not reapply the same account with a fresh epoch")
	}
	a.snapMu.Lock()
	a.snap = fresh
	a.snapMu.Unlock()
	completed() // UI result arrives after the account generation changed.
	if a.panelState.cost.summary != nil {
		t.Fatalf("completed old account detail revived: tokens=%d", a.panelState.cost.summary.Totals.TotalTokens)
	}
	tt.Frame()
	finishDetail(t, queue)
	tt.Frame()
	if a.panelState.cost.summary == nil || a.panelState.cost.summary.Totals.TotalTokens != 222 {
		t.Fatal("same-account new generation did not reload fresh detail")
	}
	if tt.HasText("111") {
		t.Fatal("old cost is still rendered")
	}
	rt.closePreferences()
}

func TestPanelRuntimeRefreshUpdatesOpenCostDetail(t *testing.T) {
	queue := detailMainQueue(t)
	p := quietLivePreferences()
	p.Language = store.LanguageEnglish
	p.ShowsCostSummary = true
	p.APICostSummaryRange = store.CostRangeCurrent
	var tokens atomic.Int64
	var scans atomic.Int32
	tokens.Store(111)
	quotaStarted, releaseQuota := make(chan struct{}), make(chan struct{})
	var quotaSignal sync.Once
	auth := codex.APIKeyAuth("synthetic-refresh")
	now := time.Now()
	svc := runway.New(runway.Options{Features: personal.Features{Grok: true, Claude: true}, Preferences: p, Now: func() time.Time { return now }, Sources: runway.Sources{
		LoadIndex:     func() (codex.AccountIndex, error) { return codex.AccountIndex{ActiveAccountID: "ada"}, nil },
		LoadValidAuth: func(context.Context, bool, *codex.Auth) (codex.Auth, error) { return auth, nil },
		FetchQuota: func(context.Context, codexapi.Auth) (codexapi.QuotaSnapshot, error) {
			if tokens.Load() != 111 {
				quotaSignal.Do(func() { close(quotaStarted) })
				<-releaseQuota
			}
			return codexapi.QuotaSnapshot{Secondary: &codexapi.RateWindow{WindowMinutes: 7 * 24 * 60, ResetsAt: now.Add(24 * time.Hour)}}, nil
		},
		FetchResetCredits: func(context.Context, codexapi.Auth) (codexapi.ResetCreditsSnapshot, error) {
			return codexapi.ResetCreditsSnapshot{}, nil
		},
		ScanCost: func(_ context.Context, queries []usage.Query, _ time.Time, _ usage.RefreshPolicy, _ *usage.ProgressReporter) (map[string]usage.Summary, error) {
			scans.Add(1)
			out := map[string]usage.Summary{}
			for _, q := range queries {
				out[q.ID] = usage.Summary{Window: q.Window, Totals: usage.Totals{TotalTokens: tokens.Load()}, CalculatedAt: now}
			}
			return out, nil
		},
	}})
	defer svc.Close()
	svc.ReloadAccounts()
	select {
	case <-svc.RefreshCost(usage.RefreshForce):
	case <-time.After(time.Second):
		t.Fatal("initial cost did not load")
	}
	rt := &liveApp{svc: svc}
	a, tt := newPanel(t, func(prefs *store.Preferences) { *prefs = p }, svc.Snapshot(), PanelActions{})
	a.SetPanelActions(rt.panelActions(a.settings))
	if err := tt.Click(a.L().Text(l10n.KeyShowDetails)); err != nil {
		t.Fatal(err)
	}
	finishDetail(t, queue)
	tt.Frame()
	if a.panelState.cost.summary == nil || a.panelState.cost.summary.Totals.TotalTokens != 111 {
		t.Fatal("initial cost did not render")
	}
	tokens.Store(222)
	svc.SetPanelOpen(true)     // Only the open panel's refresh rescans its cost.
	a.PanelActions().Refresh() // A background/runtime refresh while the detail is open.
	select {
	case <-quotaStarted:
	case <-time.After(time.Second):
		t.Fatal("real refresh did not start")
	}
	// The open detail keeps its current result while the runtime refresh
	// is blocked; a background refresh must not create a redundant UI query.
	if a.panelState.cost.summary.Totals.TotalTokens != 111 {
		t.Fatal("pending refresh replaced current detail")
	}
	select {
	case <-queue:
		t.Fatal("runtime refresh unnecessarily queried detail")
	default:
	}
	close(releaseQuota)
	rt.preferenceWork.idle.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	fresh := svc.Snapshot()
	if fresh.Codex.DisplayedCost == nil || fresh.Codex.DisplayedCost.Totals.TotalTokens != 222 {
		t.Fatal("real refresh did not produce new summary")
	}
	a.snapMu.Lock()
	a.snap = fresh
	a.snapMu.Unlock()
	tt.Frame()
	if a.panelState.cost.summary == nil || a.panelState.cost.summary.Totals.TotalTokens != 222 {
		t.Fatalf("open detail kept old result after refresh: %v", a.panelState.cost.summary)
	}
	// Real service preference/tick publications keep the same immutable Cost
	// pointer; none should refetch or reset this current-cycle detail.
	beforeScans, beforeGeneration := scans.Load(), a.panelState.cost.generation
	for i := 0; i < 5; i++ {
		prefs := svc.Preferences()
		prefs.AccountPrivacyModeEnabled = !prefs.AccountPrivacyModeEnabled
		svc.SetPreferences(prefs)
		svc.Tick(now)
		published := svc.Snapshot()
		if published.Codex.Cost != fresh.Codex.Cost {
			t.Fatal("unrelated publication copied current cost")
		}
		a.snapMu.Lock()
		a.snap = published
		a.snapMu.Unlock()
		tt.Frame()
		if a.panelState.cost.task.Running || a.panelState.cost.generation != beforeGeneration || a.panelState.cost.summary.Totals.TotalTokens != 222 {
			t.Fatal("unchanged data restarted or flashed detail")
		}
	}
	if scans.Load() != beforeScans {
		t.Fatal("unrelated publications rescanned cost")
	}
	if err := tt.Click(a.L().Text(l10n.KeyCustomRange)); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(a.L().Text(l10n.KeyCalculate)); err != nil {
		t.Fatal(err)
	}
	finishDetail(t, queue)
	tt.Frame()
	custom := a.panelState.cost.summary
	if custom == nil || a.panelState.cost.query.Custom == nil {
		t.Fatal("custom range query did not complete")
	}
	tokens.Store(333)
	select {
	case <-svc.RefreshCost(usage.RefreshForce):
	case <-time.After(time.Second):
		t.Fatal("later core cost did not load")
	}
	published := svc.Snapshot()
	a.snapMu.Lock()
	a.snap = published
	a.snapMu.Unlock()
	tt.Frame()
	if a.panelState.cost.summary != custom || a.panelState.cost.selected != 4 {
		t.Fatal("core cycle publication overwrote selected custom detail")
	}
	rt.closePreferences()
}
